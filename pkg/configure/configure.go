package configure

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/quay/quay-operator/api/v1"
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

		quayRegistry := v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      reconfigureRequest.QuayRegistryName,
				Namespace: reconfigureRequest.Namespace,
			},
			Spec: v1.QuayRegistrySpec{
				ConfigBundleSecret: newSecret.GetName(),
			},
		}
		if err := k8sClient.Patch(context.Background(), &quayRegistry, client.Merge); err != nil {
			log.Error(err, "failed to update QuayRegistry with new `configBundleSecret`: "+reconfigureRequest.Namespace+"/"+reconfigureRequest.QuayRegistryName)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// FIXME(alecmerdler): Better response body
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
		log.Println("including cert in secret: " + fullFilePathname)
		certName := strings.Split(fullFilePathname, "/")[len(strings.Split(fullFilePathname, "/"))-1]
		secretData["extra_ca_cert_"+certName] = encodedCert
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
