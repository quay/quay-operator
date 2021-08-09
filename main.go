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
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

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

const (
	operatorPort = 7071
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = quay.AddToScheme(scheme)
	_ = redhatcop.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var namespace string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", "", "The Kubernetes namespace that the controller will watch.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "7daa4ab6.quay.redhat.com",
		Namespace:          namespace,
		ClientDisableCacheFor: []client.Object{
			&quay.QuayRegistry{},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var mtx sync.Mutex
	if err = (&quaycontroller.QuayRegistryReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("controllers").WithName("QuayRegistry"),
		Scheme:         mgr.GetScheme(),
		EventRecorder:  mgr.GetEventRecorderFor("quayregistry-controller"),
		WatchNamespace: namespace,
		Mtx:            &mtx,
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

	if err = (&redhatcopcontroller.QuayEcosystemReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("QuayEcosystem"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "QuayEcosystem")
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
