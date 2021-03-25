package configure

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/quay/quay-operator/pkg/kustomize"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
)

const (
	configBundleSecretName = "quay-config-bundle"
)

// request is the expected shape of the data being sent to the reconfiguration endpoint.
type request struct {
	Config           map[string]interface{} `json:"config.yaml"`
	Certs            map[string][]byte      `json:"certs"`
	Namespace        string                 `json:"namespace"`
	QuayRegistryName string                 `json:"quayRegistryName"`
}

// response is the shape of the data returned from the reconfiguration endpoint.
type response struct {
	Status string `json:"status,omitempty"`
}

// ReconfigureHandler listens for HTTP requests containing a reconfiguration bundle from config-tool,
// creates a new k8s `Secret`, and updates the associated `QuayRegistry` to trigger a re-deployment.
func ReconfigureHandler(k8sClient client.Client) func(w http.ResponseWriter, r *http.Request) {
	log := ctrl.Log.WithName("server").WithName("Reconfigure")

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}

		var reconfigureRequest request
		err := json.NewDecoder(r.Body).Decode(&reconfigureRequest)
		if err != nil {
			log.Error(err, "failed to decode request body")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		newSecret := createUpdatedSecret(reconfigureRequest)
		if err = k8sClient.Create(context.Background(), &newSecret); err != nil {
			log.Error(err, "failed to create new config bundle secret")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Info("created new config secret for QuayRegistry: " + reconfigureRequest.Namespace + "/" + reconfigureRequest.QuayRegistryName)

		var quay v1.QuayRegistry
		if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: reconfigureRequest.Namespace, Name: reconfigureRequest.QuayRegistryName}, &quay); err != nil {
			log.Error(err, "failed to fetch QuayRegistry", "name", reconfigureRequest.QuayRegistryName, "namespace", reconfigureRequest.Namespace)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Infer managed/unmanaged components from the given `config.yaml`.
		newComponents := []v1.Component{}
		for _, component := range quay.Spec.Components {
			contains, err := kustomize.ContainsComponentConfig(reconfigureRequest.Config, component)

			if err != nil {
				log.Error(err, "failed to check `config.yaml` for component fieldgroup", "component", component.Kind)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if contains {
				log.Info("marking component as unmanaged", "component", component.Kind)
				newComponents = append(newComponents, v1.Component{Kind: component.Kind, Managed: false})
			} else {
				log.Info("marking component as managed", "component", component.Kind)
				newComponents = append(newComponents, v1.Component{Kind: component.Kind, Managed: true})
			}
		}
		quay.Spec.Components = newComponents
		quay.Spec.ConfigBundleSecret = newSecret.GetName()

		if err := k8sClient.Update(context.Background(), &quay); err != nil {
			log.Error(err, "failed to update QuayRegistry with new `configBundleSecret`: "+reconfigureRequest.Namespace+"/"+reconfigureRequest.QuayRegistryName)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// FIXME: Better response body
		js, err := json.Marshal(response{Status: "success"})
		if err != nil {
			log.Error(err, "failed to marshal response to JSON")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if _, err := w.Write(js); err != nil {
			log.Error(err, "failed to write response body")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func createUpdatedSecret(reconfigureRequest request) corev1.Secret {
	secretData := make(map[string][]byte)

	if len(reconfigureRequest.Namespace) == 0 {
		panic("namespace not provided")
	}

	if len(reconfigureRequest.QuayRegistryName) == 0 {
		panic("quayRegistryName not provided")
	}

	secretData["config.yaml"] = encode(reconfigureRequest.Config)
	for fullFilePathname, encodedCert := range reconfigureRequest.Certs {
		certName := strings.Split(fullFilePathname, "/")[len(strings.Split(fullFilePathname, "/"))-1]
		if strings.HasPrefix(fullFilePathname, "extra_ca_certs/") {
			certName = "extra_ca_cert_" + strings.ReplaceAll(certName, "extra_ca_cert_", "")
		}
		secretData[certName] = encodedCert

		log.Println("including cert in secret: " + certName)
	}

	newSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: reconfigureRequest.QuayRegistryName + "-" + configBundleSecretName + "-",
			Namespace:    reconfigureRequest.Namespace,
			Labels: map[string]string{
				"quay-registry": reconfigureRequest.QuayRegistryName,
			},
		},
		Data: secretData,
	}

	return newSecret
}

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)

	return yamlified
}
