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
	"context"
	"flag"
	"net/http"
	"os"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	quay "github.com/quay/quay-operator/apis/quay/v1"
	redhatcop "github.com/quay/quay-operator/apis/redhatcop/v1alpha1"
	quaycontroller "github.com/quay/quay-operator/controllers/quay"
	redhatcopcontroller "github.com/quay/quay-operator/controllers/redhatcop"
	"github.com/quay/quay-operator/pkg/configure"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = quay.AddToScheme(scheme)
	_ = redhatcop.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

// Arguments holds all accepted command line arguments and flags.
type Arguments struct {
	MetricsAddress     string
	ReconfigureAddress string
	Namespace          string
	LeaderElection     bool
}

// Parse parses command line arguments and flags.
func (a *Arguments) Parse() {
	flag.StringVar(
		&a.MetricsAddress,
		"metrics-addr",
		":8080",
		"The address the metric endpoint binds to.",
	)
	flag.StringVar(
		&a.ReconfigureAddress,
		"reconfigure-addr",
		":7071",
		"The address the reconfigure endpoint binds to.",
	)
	flag.BoolVar(
		&a.LeaderElection,
		"enable-leader-election",
		false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.",
	)
	flag.StringVar(
		&a.Namespace,
		"namespace",
		"",
		"The Kubernetes namespace that the controller will watch.",
	)
	flag.Parse()
}

func main() {
	var args Arguments
	args.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	mgr, err := ctrl.NewManager(
		ctrl.GetConfigOrDie(),
		ctrl.Options{
			Scheme:                scheme,
			MetricsBindAddress:    args.MetricsAddress,
			Port:                  9443,
			LeaderElection:        args.LeaderElection,
			LeaderElectionID:      "7daa4ab6.quay.redhat.com",
			Namespace:             args.Namespace,
			ClientDisableCacheFor: []client.Object{&quay.QuayRegistry{}},
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var mtx sync.Mutex
	qyrec := &quaycontroller.QuayRegistryReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("QuayRegistry"),
		Scheme:         mgr.GetScheme(),
		EventRecorder:  mgr.GetEventRecorderFor("quayregistry-controller"),
		WatchNamespace: args.Namespace,
		Mtx:            &mtx,
		ReEnqueueOnError: ctrl.Result{
			RequeueAfter: 20 * time.Second,
		},
		ReEnqueueOnSuccess: ctrl.Result{
			RequeueAfter: time.Minute,
		},
	}
	if err = qyrec.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "QuayRegistry")
		os.Exit(1)
	}

	qstrec := &quaycontroller.QuayRegistryStatusReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("QuayRegistryStatus"),
		Mtx:    &mtx,
	}
	if err = qstrec.SetupWithManager(mgr); err != nil {
		setupLog.Error(
			err, "unable to create controller", "controller", "QuayRegistryStatus",
		)
		os.Exit(1)
	}

	rhrec := &redhatcopcontroller.QuayEcosystemReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("QuayEcosystem"),
		Scheme: mgr.GetScheme(),
	}
	if err = rhrec.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "QuayEcosystem")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	ctx := ctrl.SetupSignalHandler()
	go setupReconfigureEndpoint(ctx, args.ReconfigureAddress, mgr.GetClient())

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// setupReconfigureEndpoint sets up an http server to deal with reconfigure requests made by the
// quay config tool. This function only returns when provided context has been "cancelled".
func setupReconfigureEndpoint(ctx context.Context, addr string, cli client.Client) {
	setupLog = ctrl.Log.WithName("setup").WithName("reconfigure")
	setupLog.Info("starting reconfigure endpoint server", "address", addr)

	hfunc := configure.ReconfigureHandler(cli)
	server := &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(hfunc),
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			setupLog.Error(err, "error shutting down reconfigure endpoint server")
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		setupLog.Error(err, "error shutting down reconfigure endpoint server")
		return
	}
	setupLog.Info("reconfigure server has been shutdown")
}
