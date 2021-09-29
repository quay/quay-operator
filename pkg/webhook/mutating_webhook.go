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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/meta"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	"github.com/quay/quay-operator/pkg/kustomize"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ admission.Handler = &QuayRegistryMutator{}
var _ inject.Client = &QuayRegistryMutator{}
var _ admission.DecoderInjector = &QuayRegistryMutator{}

// QuayRegistryMutator implements `admission.Handler` directly so we can use a k8s client.
type QuayRegistryMutator struct {
	client  client.Client
	decoder *admission.Decoder
}

func (m *QuayRegistryMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx).
		WithName("mutator").
		WithValues("uid", req.UID)
	ctx = logf.IntoContext(ctx, log)
	log.Info("examining object",
		"namespace", req.Namespace,
		"name", req.Name,
		"kind", req.Kind)

	// TODO(alecmerdler): Refactor into shared function
	if req.Kind.String() != v1.GroupVersionKind.String() {
		log.Info("rejecting incorrect resource kind", "groupVersionKind", req.Kind.String())

		return admission.Errored(http.StatusBadRequest, errBadKind)
	}

	var quay v1.QuayRegistry
	if err := m.decoder.Decode(req, &quay); err != nil {
		log.Error(err, "failed to decode object as `QuayRegistry`")

		return admission.Errored(http.StatusBadRequest, err)
	}

	// TODO(alecmerdler): Populate default `spec.components` based on feature detection...

	// TODO(alecmerdler): Populate `spec.configBundleSecret` if not provided...
	if quay.Spec.ConfigBundleSecret == "" {
		log.Info("`spec.configBundleSecret` is unset. Creating base `Secret`")

		baseConfigBundle, err := v1.EnsureOwnerReference(&quay, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: quay.GetName() + "-config-bundle-",
				Namespace:    quay.GetNamespace(),
			},
			Data: map[string][]byte{
				"config.yaml": encode(kustomize.BaseConfig()),
			},
		})
		if err != nil {
			msg := fmt.Sprintf("unable to add owner reference to base config bundle `Secret`: %s", err)

			return admission.Denied(msg)
		}

		if err := m.client.Create(ctx, baseConfigBundle); err != nil {
			msg := fmt.Sprintf("unable to create base config bundle `Secret`: %s", err)

			return admission.Denied(msg)
		}

		updatedQuay := quay.DeepCopy()
		objectMeta, _ := meta.Accessor(baseConfigBundle)
		updatedQuay.Spec.ConfigBundleSecret = objectMeta.GetName()
		if err := m.client.Update(ctx, updatedQuay); err != nil {
			msg := fmt.Sprintf("unable to update `spec.configBundleSecret`: %s", err)
			admission.Denied(msg)
		}

		log.Info("successfully updated `spec.configBundleSecret`")

		quayBytes, err := json.Marshal(quay)
		if err != nil {
			admission.Denied(err.Error())
		}
		updatedQuayBytes, err := json.Marshal(updatedQuay)
		if err != nil {
			admission.Denied(err.Error())
		}
		return admission.PatchResponseFromRaw(quayBytes, updatedQuayBytes)
	}

	return admission.Allowed("TODO(alecmerdler): Not implemented")
}

// InjectClient implements inject.Client.
func (m *QuayRegistryMutator) InjectClient(cl client.Client) error {
	m.client = cl
	return nil
}

// InjectDecoder implements admission.DecoderInjector.
func (m *QuayRegistryMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)

	return yamlified
}
