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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

const (
	SupportsRoutesAnnotation  = "supports-routes"
	ClusterHostnameAnnotation = "router-canonical-hostname"

	SupportsObjectStorageAnnotation = "supports-object-storage"
	StorageHostnameAnnotation       = "storage-hostname"
	StorageBucketNameAnnotation     = "storage-bucketname"
	StorageAccessKeyAnnotation      = "storage-access-key"
	StorageSecretKeyAnnotation      = "storage-secret-key"
)

type QuayVersion string

const (
	QuayVersionCurrent  QuayVersion = "vader"
	QuayVersionPrevious QuayVersion = ""
)

// CanUpgrade returns true if the given version can be upgraded to this Operator's synchronized Quay version.
func CanUpgrade(fromVersion QuayVersion) bool {
	return fromVersion == QuayVersionCurrent || fromVersion == QuayVersionPrevious
}

var allComponents = []string{
	"postgres",
	"clair",
	"redis",
	"horizontalpodautoscaler",
	"objectstorage",
	"route",
	"mirror",
}

// QuayRegistrySpec defines the desired state of QuayRegistry.
type QuayRegistrySpec struct {
	// ConfigBundleSecret is the name of the Kubernetes `Secret` in the same namespace which contains the base Quay config and extra certs.
	ConfigBundleSecret string `json:"configBundleSecret,omitempty"`
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
	// LastUpdate is the timestamp when the Operator last processed this instance.
	LastUpdate string `json:"lastUpdated,omitempty"`
	// ConfigEditorEndpoint is the external access point for a web-based reconfiguration interface
	// for the Quay registry instance.
	ConfigEditorEndpoint string `json:"configEditorEndpoint,omitempty"`
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
		if component.Kind == "objectstorage" && component.Managed && !supportsObjectBucketClaims(quay) {
			return nil, errors.New("cannot use `objectstorage` component when `ObjectBucketClaims` API not available")
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
				continue
			}
			if component == "objectstorage" && !supportsObjectBucketClaims(quay) {
				continue
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

// EnsureRegistryEndpoint sets the `status.registryEndpoint` field and returns `ok` if it was changed.
func EnsureRegistryEndpoint(quay *QuayRegistry) (*QuayRegistry, bool) {
	updatedQuay := quay.DeepCopy()

	if supportsRoutes(quay) {
		clusterHostname := quay.GetAnnotations()[ClusterHostnameAnnotation]
		// FIXME(alecmerdler): This is incorrect if `SERVER_HOSTNAME` is set...
		updatedQuay.Status.RegistryEndpoint = strings.Join([]string{
			strings.Join([]string{quay.GetName(), "quay", quay.GetNamespace()}, "-"),
			clusterHostname},
			".")
	}
	// TODO(alecmerdler): Retrieve load balancer IP from `Service`

	return updatedQuay, quay.Status.RegistryEndpoint == updatedQuay.Status.RegistryEndpoint
}

// EnsureConfigEditorEndpoint sets the `status.configEditorEndpoint` field and returns `ok` if it was changed.
func EnsureConfigEditorEndpoint(quay *QuayRegistry) (*QuayRegistry, bool) {
	updatedQuay := quay.DeepCopy()

	if supportsRoutes(quay) {
		clusterHostname := quay.GetAnnotations()[ClusterHostnameAnnotation]
		updatedQuay.Status.ConfigEditorEndpoint = strings.Join([]string{
			strings.Join([]string{quay.GetName(), "quay-config-editor", quay.GetNamespace()}, "-"),
			clusterHostname},
			".")
	}
	// TODO(alecmerdler): Retrieve load balancer IP from `Service`

	return updatedQuay, quay.Status.ConfigEditorEndpoint == updatedQuay.Status.ConfigEditorEndpoint
}

// EnsureOwnerReference adds an `ownerReference` to the given object if it does not already have one.
func EnsureOwnerReference(quay *QuayRegistry, obj runtime.Object) (runtime.Object, error) {
	objectMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	hasOwnerRef := false
	for _, ownerRef := range objectMeta.GetOwnerReferences() {
		if ownerRef.Name == quay.GetName() &&
			ownerRef.Kind == "QuayRegistry" &&
			ownerRef.APIVersion == GroupVersion.String() &&
			ownerRef.UID == quay.UID {
			hasOwnerRef = true
		}
	}

	if !hasOwnerRef {
		objectMeta.SetOwnerReferences(append(objectMeta.GetOwnerReferences(), metav1.OwnerReference{
			APIVersion: GroupVersion.String(),
			Kind:       "QuayRegistry",
			Name:       quay.GetName(),
			UID:        quay.GetUID(),
		}))
	}

	return obj, nil
}

func supportsRoutes(quay *QuayRegistry) bool {
	annotations := quay.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	_, ok := annotations[SupportsRoutesAnnotation]

	return ok
}

func supportsObjectBucketClaims(quay *QuayRegistry) bool {
	annotations := quay.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	_, ok := annotations[SupportsObjectStorageAnnotation]

	return ok
}

func init() {
	SchemeBuilder.Register(&QuayRegistry{}, &QuayRegistryList{})
}
