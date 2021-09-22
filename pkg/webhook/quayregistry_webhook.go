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
	"errors"
	"net/http"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var errBadKind = errors.New("request object is not `QuayRegistry`")

// SetupWebhooks registers custom admission webhooks with the manager.
func SetupWebhooks(mgr ctrl.Manager) error {
	log := mgr.GetLogger().WithName("quayregistry-admission")

	injectLogger := func(ctx context.Context, _ *http.Request) context.Context {
		return logf.IntoContext(ctx, log)
	}

	log.Info("registering admission webhooks")

	webhookServer := mgr.GetWebhookServer()
	webhookServer.Register("/validate", &webhook.Admission{
		Handler:         &QuayRegistryValidator{client: mgr.GetClient()},
		WithContextFunc: injectLogger,
	})
	webhookServer.Register("/mutate", &webhook.Admission{
		Handler:         &QuayRegistryMutator{client: mgr.GetClient()},
		WithContextFunc: injectLogger,
	})

	return nil
}
