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

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ admission.Handler = &QuayRegistryValidator{}
var _ inject.Client = &QuayRegistryValidator{}
var _ admission.DecoderInjector = &QuayRegistryValidator{}

// QuayRegistryValidator implements `admission.Handler` directly so we can use a k8s client.
type QuayRegistryValidator struct {
	client  client.Client
	decoder *admission.Decoder
}

func (v *QuayRegistryValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := logf.FromContext(ctx).
		WithName("validator").
		WithValues("uid", req.UID)
	ctx = logf.IntoContext(ctx, log)
	log.Info("examining object",
		"namespace", req.Namespace,
		"name", req.Name,
		"kind", req.Kind)
	// TODO(alecmerdler): Validate `spec.components` based on feature detection...

	// TODO(alecmerdler): Validate `spec.configBundleSecret`, check for (un)managed components config values, check provided TLS cert/key pair, etc...

	return admission.Allowed("TODO(alecmerdler): Not implemented")
}

// InjectClient implements inject.Client.
func (v *QuayRegistryValidator) InjectClient(cl client.Client) error {
	v.client = cl
	return nil
}

// InjectDecoder implements admission.DecoderInjector.
func (v *QuayRegistryValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
