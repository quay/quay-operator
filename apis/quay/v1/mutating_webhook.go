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

package v1

import (
	"context"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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
	if req.Kind.String() != GroupVersionKind.String() {
		log.Info("rejecting incorrect resource kind", "groupVersionKind", req.Kind.String())

		return admission.Errored(http.StatusBadRequest, errBadKind)
	}

	var quay QuayRegistry
	if err := m.decoder.Decode(req, &quay); err != nil {
		log.Error(err, "failed to decode object as `QuayRegistry`")

		return admission.Errored(http.StatusBadRequest, err)
	}

	// TODO(alecmerdler): Populate default `spec.components` based on feature detection...

	// TODO(alecmerdler): Populate `spec.configBundleSecret` if not provided...

	return admission.Denied("TODO(alecmerdler): Not implemented")
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
