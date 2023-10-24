/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	quay "github.com/quay/quay-operator/apis/quay/v1"
	quaycontroller "github.com/quay/quay-operator/controllers/quay"
	"github.com/quay/quay-operator/pkg/configure"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	operatorPort = 7071
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = quay.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	enableHTTP2 := false
	metricsAddr := ":8080"
	secureMetrics := false
	enableLeaderElection := false
	namespace := ""
	flag.BoolVar(&enableHTTP2, "enable-http2", enableHTTP2, "If HTTP/2 should be enabled for the metrics and webhook servers.")
	flag.StringVar(&metricsAddr, "metrics-addr", metricsAddr, "The address the metric endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", secureMetrics, "If the metrics endpoint should be served securely.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", enableLeaderElection,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", namespace, "The Kubernetes namespace that the controller will watch.")
	flag.Parse()

	// if this environment variable is set the operator removes all resource requirements
	// (requests and limits), this is useful for development purposes.
	skipres := os.Getenv("SKIP_RESOURCE_REQUESTS") == "true"

	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.JSONEncoder(func(o *zapcore.EncoderConfig) {
		o.EncodeTime = zapcore.RFC3339TimeEncoder
	})))

	ctrl.Log.Info("Starting the Quay Operator", "namespace", namespace)

	disableHTTP2 := func(c *tls.Config) {
		if enableHTTP2 {
			return
		}
		c.NextProtos = []string{"http/1.1"}
	}

	cacheOptions := cache.Options{}
	if namespace != "" {
		cacheOptions.DefaultNamespaces = map[string]cache.Config{
			namespace: {},
		}
	}

	clientOptions := client.Options{
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&quay.QuayRegistry{},
			},
		},
	}

	metricsOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       []func(*tls.Config){disableHTTP2},
	}

	webhookServerOptions := webhook.Options{
		Port:    9443,
		TLSOpts: []func(config *tls.Config){disableHTTP2},
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		Cache:            cacheOptions,
		Client:           clientOptions,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "7daa4ab6.quay.redhat.com",
		Metrics:          metricsOptions,
		WebhookServer:    webhookServer,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var mtx sync.Mutex
	if err = (&quaycontroller.QuayRegistryReconciler{
		Client:               mgr.GetClient(),
		Log:                  ctrl.Log.WithName("controllers").WithName("QuayRegistry"),
		Scheme:               mgr.GetScheme(),
		EventRecorder:        mgr.GetEventRecorderFor("quayregistry-controller"),
		WatchNamespace:       namespace,
		Mtx:                  &mtx,
		Requeue:              ctrl.Result{RequeueAfter: 10 * time.Second},
		SkipResourceRequests: skipres,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "QuayRegistry")
		os.Exit(1)
	}

	if err = (&quaycontroller.QuayRegistryStatusReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("QuayRegistryStatus"),
		Mtx:    &mtx,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "QuayRegistryStatus")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting server on port 7071")
	go func() {
		http.HandleFunc("/reconfigure", configure.ReconfigureHandler(mgr.GetClient()))
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", operatorPort), nil))
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
