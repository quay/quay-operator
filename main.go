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

	jsoniter "github.com/json-iterator/go"
	quayredhatcomv1 "github.com/quay/quay-operator/api/v1"
	"github.com/quay/quay-operator/controllers"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = quayredhatcomv1.AddToScheme(scheme)
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
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.QuayRegistryReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("QuayRegistry"),
		Scheme: mgr.GetScheme(),
		Config: mgr.GetConfig(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "QuayRegistry")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	http.HandleFunc("/reconfigure", reconfigureHandler(mgr))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", 7071), nil))

	// setupLog.Info("starting manager")
	// if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
	// 	setupLog.Error(err, "problem running manager")
	// 	os.Exit(1)
	// }
}

// handler to listen for reconfiguration bundle from config-tool
func reconfigureHandler(mgr manager.Manager) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(404)
			return
		}

		var preSecret map[string]interface{}
		err := yaml.NewDecoder(r.Body).Decode(&preSecret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		newSecret := createUpdatedSecret(preSecret, "NAME", "NAMESPACE") // NEED TO GET NAMESPACE AND NAME SOMEHOW
		fmt.Println(newSecret.Data)
		response := map[string]string{"status": "success"} // TEMP RESPONSE
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		js, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.Write(js)
	}

}

func createUpdatedSecret(preSecret map[string]interface{}, name, namespace string) corev1.Secret {

	secretData := make(map[string][]byte)
	for file, contents := range preSecret {
		if file == "config.yaml" {
			secretData[file] = encode(contents)
		} else {
			secretData[file] = contents.([]byte)
		}

	}

	newSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: secretData,
	}

	return newSecret
}

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)

	return yamlified
}
