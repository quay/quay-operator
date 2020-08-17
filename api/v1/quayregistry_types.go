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
	"errors"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type QuayVersion string

const ClusterHostnameAnnotation = "router-canonical-hostname"

const (
	QuayVersionQuiGon QuayVersion = "qui-gon"
	QuayVersionPadme  QuayVersion = "padme"
)

var quayVersions = []QuayVersion{
	QuayVersionPadme,
	QuayVersionQuiGon,
}

var allComponents = []string{
	"postgres",
	"clair",
	"redis",
	"localstorage",
	"route",
}

// QuayRegistrySpec defines the desired state of QuayRegistry.
type QuayRegistrySpec struct {
	// DesiredVersion declares the version of Quay that should deployed and managed.
	// Upgrading Quay is accomplished by modifying this field. Runtime validation will prevent upgrading backwards.
	// If any unmanaged components are incompatible with the value of this field, the Operator will not upgrade.
	// If omitted, will default to the latest version that the Operator knows how to manage.
	DesiredVersion QuayVersion `json:"desiredVersion,omitempty"`
	// ConfigBundleSecret is the name of the Kubernetes `Secret` in the same namespace which contains the base Quay config and extra certs.
	ConfigBundleSecret string `json:"configBundleSecret"`
	// Components declare how the Operator should handle backing Quay services.
	Components []Component `json:"components,omitempty"`
}

// Component describes how the Operator should handle a backing Quay service.
type Component struct {
	// Kind is the unique name of this type of component.
	Kind string `json:"kind"`
	// Managed indicates whether or not the Operator is responsible for the lifecycle of this component.
	// Default is true.
	Managed bool `json:"managed"`
}

// QuayRegistryStatus defines the observed state of QuayRegistry.
type QuayRegistryStatus struct {
	// CurrentVersion is the actual version of Quay that is actively deployed.
	CurrentVersion QuayVersion `json:"currentVersion,omitempty"`
	// RegistryEndpoint is the external access point for the Quay registry.
	RegistryEndpoint string `json:"registryEndpoint,omitempty"`
	// TODO(alecmerdler): Config editor endpoint
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// QuayRegistry is the Schema for the quayregistries API.
type QuayRegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuayRegistrySpec   `json:"spec,omitempty"`
	Status QuayRegistryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QuayRegistryList contains a list of QuayRegistry.
type QuayRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuayRegistry `json:"items"`
}

// EnsureDefaultComponents adds any `Components` which are missing from `Spec.Components`
// and returns a new `QuayRegistry` copy.
func EnsureDefaultComponents(quay *QuayRegistry) (*QuayRegistry, error) {
	updatedQuay := quay.DeepCopy()
	if updatedQuay.Spec.Components == nil {
		updatedQuay.Spec.Components = []Component{}
	}

	for _, component := range quay.Spec.Components {
		if component.Kind == "route" && component.Managed && !supportsRoutes(quay) {
			return nil, errors.New("cannot use `route` component when `Route` API not available")
		}
	}

	for _, component := range allComponents {
		found := false
		for _, definedComponent := range quay.Spec.Components {
			if component == definedComponent.Kind {
				found = true
				break
			}
		}

		if !found {
			if component == "route" && !supportsRoutes(quay) {
				break
			}

			updatedQuay.Spec.Components = append(updatedQuay.Spec.Components, Component{Kind: component, Managed: true})
		}
	}

	return updatedQuay, nil
}

// ComponentsMatch returns true if both set of components are equivalent, and false otherwise.
func ComponentsMatch(firstComponents, secondComponents []Component) bool {
	if len(firstComponents) != len(secondComponents) {
		return false
	}

	for _, compA := range firstComponents {
		equal := false
		for _, compB := range secondComponents {
			if compA.Kind == compB.Kind && compA.Managed == compB.Managed {
				equal = true
				break
			}
		}
		if !equal {
			return false
		}
	}
	return true
}

// EnsureDesiredVersion validates that the Operator can managed the `Spec.DesiredVersion` indicated,
// or else sets it to the latest version it can manage if unset.
func EnsureDesiredVersion(quay *QuayRegistry) (*QuayRegistry, error) {
	updatedQuay := quay.DeepCopy()

	if updatedQuay.Spec.DesiredVersion == "" {
		updatedQuay.Spec.DesiredVersion = quayVersions[len(quayVersions)-1]

		return updatedQuay, nil
	}

	for _, version := range quayVersions {
		if version == updatedQuay.Spec.DesiredVersion {
			return updatedQuay, nil
		}
	}

	// TODO(alecmerdler): Ensure that `spec.desiredVersion` is not a "downgrade" from `status.currentVersion`

	return updatedQuay, errors.New("invalid `desiredVersion`: " + string(updatedQuay.Spec.DesiredVersion))
}

// EnsureRegistryEndpoint sets the `status.registryEndpoint` field and returns `ok` if it was changed.
func EnsureRegistryEndpoint(quay *QuayRegistry) (*QuayRegistry, bool) {
	updatedQuay := quay.DeepCopy()

	if supportsRoutes(quay) {
		clusterHostname := quay.GetAnnotations()[ClusterHostnameAnnotation]
		updatedQuay.Status.RegistryEndpoint = strings.Join([]string{
			strings.Join([]string{quay.GetName(), "quay", quay.GetNamespace()}, "-"),
			clusterHostname},
			".")
	}
	// TODO(alecmerdler): Retrieve load balancer IP from `Service`

	return updatedQuay, quay.Status.RegistryEndpoint == updatedQuay.Status.RegistryEndpoint
}

func supportsRoutes(quay *QuayRegistry) bool {
	annotations := quay.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	_, ok := annotations[ClusterHostnameAnnotation]

	return ok
}

func init() {
	SchemeBuilder.Register(&QuayRegistry{}, &QuayRegistryList{})
}
