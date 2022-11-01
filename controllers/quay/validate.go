package controllers

import (
	"fmt"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	"github.com/quay/quay-operator/pkg/kustomize"
)

// validateHasNecessaryConfig checks every component has been provided with a proper configuration. For
// instance if redis is unmanaged the user provided configuration must contain a custom redis
// config otherwise we may render an invalid quay deployment.
func (r *QuayRegistryReconciler) validateHasNecessaryConfig(
	quay v1.QuayRegistry, cfg map[string][]byte,
) error {
	for _, cmp := range quay.Spec.Components {
		hascfg, err := kustomize.ContainsComponentConfig(cfg, cmp)
		if err != nil {
			return fmt.Errorf("unable to verify component config: %w", err)
		}

		if cmp.Managed {
			// if the user has not provided config for a managed component or if the
			// managed component supports custom config even when managed we are ok.
			// if the component is marked as managed but the user has provided config
			// for it (in config bundle secret) then we have a problem.
			if !hascfg || v1.ComponentSupportsConfigWhenManaged(cmp) {
				continue
			}

			return fmt.Errorf(
				"%s component marked as managed, but `configBundleSecret` "+
					"contains required fields",
				cmp.Kind,
			)
		}

		// if the unmanaged component has been provided with proper config we can move
		// on as we already have everything we need.
		if hascfg {
			continue
		}

		// if user miss a configuration for an required component we fail.
		if v1.RequiredComponent(cmp.Kind) {
			return fmt.Errorf(
				"required component `%s` marked as unmanaged, but "+
					"`configBundleSecret` is missing necessary fields",
				cmp.Kind,
			)
		}

		// if we got here then it means the user has not provided configuration and
		// the copmonent is optional. almost all components support this scenario, the
		// exception is clairpostgres, this component can't be render correctly if clair
		// is set as managed but no database configuration has been provided to it.
		managedclair := v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentClair)
		if cmp.Kind == v1.ComponentClairPostgres && managedclair {
			return fmt.Errorf(
				"clairpostgres component unmanaged but no clair postgres " +
					"config provided",
			)
		}
	}
	return nil
}
