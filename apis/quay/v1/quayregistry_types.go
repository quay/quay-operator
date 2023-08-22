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
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	quaycontext "github.com/quay/quay-operator/pkg/context"
)

// QuayVersion represents a quay version as a string. Normally this is set using semantic
// versioning, e.g. 3.7.0.
type QuayVersion string

// QuayVersionCurrent holds the current quay version as captured by means of an environment
// variable. This can be used to identify a version upgrade.
var QuayVersionCurrent QuayVersion = QuayVersion(os.Getenv("QUAY_VERSION"))

// ComponentKind holds a component type, e.g. "clair", "postgres", etc.
type ComponentKind string

// Follow a list of constants representing all supported components.
const (
	ComponentQuay          ComponentKind = "quay"
	ComponentPostgres      ComponentKind = "postgres"
	ComponentClair         ComponentKind = "clair"
	ComponentClairPostgres ComponentKind = "clairpostgres"
	ComponentRedis         ComponentKind = "redis"
	ComponentHPA           ComponentKind = "horizontalpodautoscaler"
	ComponentObjectStorage ComponentKind = "objectstorage"
	ComponentRoute         ComponentKind = "route"
	ComponentMirror        ComponentKind = "mirror"
	ComponentMonitoring    ComponentKind = "monitoring"
	ComponentTLS           ComponentKind = "tls"
)

// AllComponents holds a list of all supported components.
var AllComponents = []ComponentKind{
	ComponentQuay,
	ComponentPostgres,
	ComponentClair,
	ComponentRedis,
	ComponentHPA,
	ComponentObjectStorage,
	ComponentRoute,
	ComponentMirror,
	ComponentMonitoring,
	ComponentTLS,
	ComponentClairPostgres,
}

var requiredComponents = []ComponentKind{
	ComponentPostgres,
	ComponentObjectStorage,
	ComponentRoute,
	ComponentRedis,
	ComponentTLS,
}

var supportsVolumeOverride = []ComponentKind{
	ComponentPostgres,
	ComponentClair,
}

var supportsEnvOverride = []ComponentKind{
	ComponentQuay,
	ComponentClair,
	ComponentMirror,
	ComponentPostgres,
	ComponentRedis,
}

var supportsReplicasOverride = []ComponentKind{
	ComponentClair,
	ComponentMirror,
	ComponentQuay,
}

var supportsAffinityOverride = []ComponentKind{
	ComponentClair,
	ComponentMirror,
	ComponentQuay,
}

const (
	ManagedKeysName             = "quay-registry-managed-secret-keys"
	QuayConfigTLSSecretName     = "quay-config-tls"
	QuayUpgradeJobName          = "quay-app-upgrade"
	PostgresUpgradeJobName      = "quay-postgres-upgrade"
	ClairPostgresUpgradeJobName = "clair-postgres-upgrade"
	ClusterServiceCAName        = "cluster-service-ca"
	ClusterTrustedCAName        = "cluster-trusted-ca"
)

// QuayRegistrySpec defines the desired state of QuayRegistry.
type QuayRegistrySpec struct {
	// ConfigBundleSecret is the name of the Kubernetes `Secret` in the same namespace
	// which contains the base Quay config and extra certs.
	ConfigBundleSecret string `json:"configBundleSecret,omitempty"`
	// Components declare how the Operator should handle backing Quay services.
	Components []Component `json:"components,omitempty"`
}

// Component describes how the Operator should handle a backing Quay service.
type Component struct {
	// Kind is the unique name of this type of component.
	Kind ComponentKind `json:"kind"`
	// Managed indicates whether or not the Operator is responsible for the lifecycle of this component.
	// Default is true.
	Managed bool `json:"managed"`
	// Overrides holds information regarding component specific configurations.
	Overrides *Override `json:"overrides,omitempty"`
}

// Override describes configuration overrides for the given managed component
type Override struct {
	VolumeSize  *resource.Quantity `json:"volumeSize,omitempty"`
	Env         []corev1.EnvVar    `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
	Replicas    *int32             `json:"replicas,omitempty"`
	Affinity    *corev1.Affinity   `json:"affinity,omitempty"`
	Labels      map[string]string  `json:"labels,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty"`
}

type ConditionType string

// Follow a list of known condition types. These are used when reporting a QuayRegistry status
// through its .status.conditions slice.
const (
	ConditionTypeAvailable      ConditionType = "Available"
	ConditionTypeRolloutBlocked ConditionType = "RolloutBlocked"
	ConditionComponentsCreated  ConditionType = "ComponentsCreated"
	ComponentQuayReady          ConditionType = "ComponentQuayReady"
	ComponentPostgresReady      ConditionType = "ComponentPostgresReady"
	ComponentClairReady         ConditionType = "ComponentClairReady"
	ComponentClairPostgresReady ConditionType = "ComponentClairPostgresReady"
	ComponentRedisReady         ConditionType = "ComponentRedisReady"
	ComponentHPAReady           ConditionType = "ComponentHPAReady"
	ComponentObjectStorageReady ConditionType = "ComponentObjectStorageReady"
	ComponentRouteReady         ConditionType = "ComponentRouteReady"
	ComponentMirrorReady        ConditionType = "ComponentMirrorReady"
	ComponentMonitoringReady    ConditionType = "ComponentMonitoringReady"
	ComponentTLSReady           ConditionType = "ComponentTLSReady"
)

type ConditionReason string

// Below follow a list of all Reasons used while reporting QuayRegistry conditions through its
// .status.conditions field.
const (
	ConditionReasonComponentNotReady    ConditionReason = "ComponentNotReady"
	ConditionReasonComponentReady       ConditionReason = "ComponentReady"
	ConditionReasonComponentUnmanaged   ConditionReason = "ComponentNotManaged"
	ConditionReasonHealthChecksPassing  ConditionReason = "HealthChecksPassing"
	ConditionReasonMigrationsInProgress ConditionReason = "MigrationsInProgress"
	ConditionReasonMigrationsFailed     ConditionReason = "MigrationsFailed"
	ConditionReasonMigrationsJobMissing ConditionReason = "MigrationsJobMissing"

	ConditionReasonPostgresUpgradeInProgress ConditionReason = "PostgresUpgradeInProgress"
	ConditionReasonPostgresUpgradeFailed     ConditionReason = "PostgresUpgradeFailed"
	ConditionReasonPostgresUpgradeJobMissing ConditionReason = "PostgresUpgradeJobMissing"

	ConditionReasonComponentsCreationSuccess             ConditionReason = "ComponentsCreationSuccess"
	ConditionReasonUpgradeUnsupported                    ConditionReason = "UpgradeUnsupported"
	ConditionReasonComponentCreationFailed               ConditionReason = "ComponentCreationFailed"
	ConditionReasonRouteComponentDependencyError         ConditionReason = "RouteComponentDependencyError"
	ConditionReasonObjectStorageComponentDependencyError ConditionReason = "ObjectStorageComponentDependencyError"
	ConditionReasonMonitoringComponentDependencyError    ConditionReason = "MonitoringComponentDependencyError"
	ConditionReasonConfigInvalid                         ConditionReason = "ConfigInvalid"
	ConditionReasonComponentOverrideInvalid              ConditionReason = "ComponentOverrideInvalid"
)

// Condition is a single condition of a QuayRegistry.
// Conditions should follow the "abnormal-true" principle in order to only bring the attention of users to "broken" states.
// Example: a condition of `type: "Ready", status: "True"“ is less useful and should be omitted whereas `type: "NotReady", status: "True"`
// is more useful when trying to monitor when something is wrong.
type Condition struct {
	Type               ConditionType          `json:"type,omitempty"`
	Status             metav1.ConditionStatus `json:"status,omitempty"`
	Reason             ConditionReason        `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
	LastUpdateTime     metav1.Time            `json:"lastUpdateTime,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
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
	// ConfigEditorCredentialsSecret is the Kubernetes `Secret` containing the config editor password.
	ConfigEditorCredentialsSecret string `json:"configEditorCredentialsSecret,omitempty"`
	// Conditions represent the conditions that a QuayRegistry can have.
	Conditions []Condition `json:"conditions,omitempty"`
}

// GetCondition retrieves the condition with the matching type from the given list.
func GetCondition(conditions []Condition, conditionType ConditionType) *Condition {
	for _, c := range conditions {
		if c.Type == conditionType {
			return &c
		}
	}

	return nil
}

// SetCondition adds or updates a given condition.
// TODO: Use https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apimachinery/pkg/api/meta/conditions.go when we can.
func SetCondition(existing []Condition, newCondition Condition) []Condition {
	if existing == nil {
		existing = []Condition{}
	}

	for i, existingCondition := range existing {
		if existingCondition.Type == newCondition.Type {
			existing[i] = newCondition
			return existing
		}
	}

	return append(existing, newCondition)
}

// RemoveCondition removes any conditions with the matching type.
func RemoveCondition(conditions []Condition, conditionType ConditionType) []Condition {
	if conditions == nil {
		return []Condition{}
	}

	filtered := []Condition{}
	for _, existingCondition := range conditions {
		if existingCondition.Type != conditionType {
			filtered = append(filtered, existingCondition)
		}
	}

	return filtered
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

// MigrationsRunning returns true if the status for provided QuayRegistry indicates that
// the database migrations are running.
func MigrationsRunning(quay *QuayRegistry) bool {
	created := GetCondition(quay.Status.Conditions, ConditionComponentsCreated)
	if created == nil {
		return false
	}
	return created.Reason == ConditionReasonMigrationsInProgress
}

// PostgresUpgradeRunning returns true if the status for provided QuayRegistry indicates that
// the database upgrade is running.
func PostgresUpgradeRunning(quay *QuayRegistry) bool {
	created := GetCondition(quay.Status.Conditions, ConditionComponentsCreated)
	if created == nil {
		return false
	}
	return created.Reason == ConditionReasonPostgresUpgradeInProgress || created.Reason == ConditionReasonPostgresUpgradeFailed
}

// FlaggedForDeletion returns a boolean indicating if provided QuayRegistry object has
// been flagged for deletion.
func FlaggedForDeletion(quay *QuayRegistry) bool {
	return quay.GetDeletionTimestamp() != nil
}

// NeedsBundleSecret returns if provided QuayRegistry has not a config bundle secret
// populated on its Spec.ConfigBundleSecret property.
func NeedsBundleSecret(quay *QuayRegistry) bool {
	return quay.Spec.ConfigBundleSecret == ""
}

// ComponentSupportsConfigWhenManaged returns true if provided component can live with
// being Managed AND containing a custom user config provided through the config bundle
// secret.
func ComponentSupportsConfigWhenManaged(cmp Component) bool {
	return cmp.Kind == ComponentRoute || cmp.Kind == ComponentMirror
}

func EnsureComponents(components []Component) []Component {
	return append(components, components[0])[1 : len(components)+1]
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

// ComponentIsManaged returns whether the given component is managed or not.
func ComponentIsManaged(components []Component, name ComponentKind) bool {
	// we do not expose Quay component through the CRD, as it represents the Quay deployment
	// and will always be managed.
	if name == ComponentQuay {
		return true
	}

	for _, c := range components {
		if c.Kind == name {
			return c.Managed
		}
	}
	return false
}

// RequiredComponent returns whether the given component is required for Quay or not.
func RequiredComponent(component ComponentKind) bool {
	for _, c := range requiredComponents {
		if c == component {
			return true
		}
	}
	return false
}

// EnsureDefaultComponents adds any `Components` which are missing from `Spec.Components`.
// Returns an error if a component was declared as managed but is not supported in the current
// k8s cluster.
func EnsureDefaultComponents(ctx *quaycontext.QuayRegistryContext, quay *QuayRegistry) error {
	if quay.Spec.Components == nil {
		quay.Spec.Components = []Component{}
	}

	type check struct {
		check func() bool
		msg   string
	}
	checks := map[ComponentKind]check{
		ComponentRoute: {
			check: func() bool { return ctx.SupportsRoutes },
			msg:   "Route API not available",
		},
		ComponentTLS: {
			check: func() bool { return ctx.SupportsRoutes },
			msg:   "Route API not available",
		},
		ComponentObjectStorage: {
			check: func() bool { return ctx.SupportsObjectStorage },
			msg:   "ObjectStorage API not available",
		},
		ComponentMonitoring: {
			check: func() bool { return ctx.SupportsMonitoring },
			msg:   "Prometheus API not available",
		},
	}

	componentManaged := map[ComponentKind]check{
		ComponentTLS: {
			check: func() bool { return ctx.TLSCert == nil && ctx.TLSKey == nil },
		},
	}

	for _, cmp := range AllComponents {
		ccheck, checkexists := checks[cmp]
		if checkexists {
			// if there is a check registered for the component we run it, if the
			// check fails and the component is managed then we have a problem with
			// the current components configuration. returns the check error.
			if !ccheck.check() && ComponentIsManaged(quay.Spec.Components, cmp) {
				return fmt.Errorf(
					"error validating component %s: %s", cmp, ccheck.msg,
				)
			}
		}

		// if the component has already been declared in the QuayRegistry object we can
		// just return as there is nothing we need to do.
		var found bool
		for i, declaredComponent := range quay.Spec.Components {
			if cmp != declaredComponent.Kind {
				continue
			}

			// we disregard whatever the user has defined for Quay component, this
			// is a component that can't be unmanaged so if user sets it to unmanaged
			// we are going to roll it back to managed.
			if declaredComponent.Kind == ComponentQuay {
				quay.Spec.Components[i].Managed = true
			}

			found = true
			break
		}
		if found {
			continue
		}

		// the component management status is set to true if the check for the component
		// has passed.
		managed := !checkexists || ccheck.check()
		if _, ok := componentManaged[cmp]; ok {
			managed = managed && componentManaged[cmp].check()
		}

		quay.Spec.Components = append(
			quay.Spec.Components,
			Component{
				Kind:    cmp,
				Managed: managed,
			},
		)
	}

	return nil
}

// hasAffinity checks if any anti/affinity option has been changed
// returns true if something has changed, false otherwise
func hasAffinity(component Component) bool {
	overrideAffinity := component.Overrides.Affinity
	return overrideAffinity != nil
}

// ValidateOverrides validates that the overrides set for each component are valid.
func ValidateOverrides(quay *QuayRegistry) error {
	for _, component := range quay.Spec.Components {

		// No overrides provided
		if component.Overrides == nil {
			continue
		}

		hasaffinity := hasAffinity(component)
		hasvolume := component.Overrides.VolumeSize != nil
		hasreplicas := component.Overrides.Replicas != nil
		hasenvvar := len(component.Overrides.Env) > 0
		hasoverride := hasaffinity || hasvolume || hasenvvar || hasreplicas

		if hasoverride && !ComponentIsManaged(quay.Spec.Components, component.Kind) {
			return fmt.Errorf("cannot set overrides on unmanaged %s", component.Kind)
		}

		if hasreplicas && ComponentIsManaged(quay.Spec.Components, ComponentHPA) {
			// with managed HPA we only accept zero as an override for the number
			// of replicas. we can't compete with HPA except when scaling down.
			if *component.Overrides.Replicas != 0 {
				return fmt.Errorf("cannot override replicas with managed HPA")
			}
		}

		// Check that component supports override
		if hasaffinity && !ComponentSupportsOverride(component.Kind, "affinity") {
			return fmt.Errorf(
				"component %s does not support affinity overrides",
				component.Kind,
			)
		}

		if hasvolume && !ComponentSupportsOverride(component.Kind, "volumeSize") {
			return fmt.Errorf(
				"component %s does not support volumeSize overrides",
				component.Kind,
			)
		}

		if hasenvvar && !ComponentSupportsOverride(component.Kind, "env") {
			return fmt.Errorf(
				"component %s does not support env overrides",
				component.Kind,
			)
		}

		if hasreplicas && !ComponentSupportsOverride(component.Kind, "replicas") {
			return fmt.Errorf(
				"component %s does not support replicas overrides",
				component.Kind,
			)
		}
	}

	return nil
}

// EnsureRegistryEndpoint sets the `status.registryEndpoint` field and returns `ok` if it was
// unchanged.
func EnsureRegistryEndpoint(
	qctx *quaycontext.QuayRegistryContext, quay *QuayRegistry, config map[string]interface{},
) bool {
	orig := quay.Status.RegistryEndpoint
	if config == nil {
		config = map[string]interface{}{}
	}

	if serverHostname, ok := config["SERVER_HOSTNAME"]; ok {
		quay.Status.RegistryEndpoint = "https://" + serverHostname.(string)
	} else if qctx.SupportsRoutes {
		quay.Status.RegistryEndpoint = fmt.Sprintf(
			"https://%s-quay-%s.%s",
			quay.GetName(),
			quay.GetNamespace(),
			qctx.ClusterHostname,
		)
	}

	return quay.Status.RegistryEndpoint == orig
}

// EnsureConfigEditorEndpoint sets the `status.configEditorEndpoint` field. If routes are
// not supported or route component is unmanaged this sets configEditorEndpoint to empty
// string.
func EnsureConfigEditorEndpoint(ctx *quaycontext.QuayRegistryContext, quay *QuayRegistry) {
	if !ctx.SupportsRoutes {
		quay.Status.ConfigEditorEndpoint = ""
		return
	}

	if !ComponentIsManaged(quay.Spec.Components, ComponentRoute) {
		quay.Status.ConfigEditorEndpoint = ""
		return
	}

	quay.Status.ConfigEditorEndpoint = fmt.Sprintf(
		"https://%s-quay-config-editor-%s.%s",
		quay.GetName(),
		quay.GetNamespace(),
		ctx.ClusterHostname,
	)
}

// Owns verifies if a QuayRegistry object owns provided Object.
func Owns(quay QuayRegistry, obj client.Object) bool {
	for _, owref := range obj.GetOwnerReferences() {
		if owref.Kind != "QuayRegistry" {
			continue
		}
		if owref.Name != quay.GetName() {
			continue
		}
		if owref.APIVersion != GroupVersion.String() {
			continue
		}
		if owref.UID != quay.UID {
			continue
		}
		return true
	}
	return false
}

// EnsureOwnerReference adds an `ownerReference` to the given object if it does not already
// have one.
func EnsureOwnerReference(quay *QuayRegistry, obj client.Object) client.Object {
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.Kind != "QuayRegistry" {
			continue
		}
		if ownerRef.APIVersion != GroupVersion.String() {
			continue
		}
		if ownerRef.Name != quay.GetName() {
			continue
		}
		if ownerRef.UID != quay.UID {
			continue
		}
		return obj
	}

	obj.SetOwnerReferences(
		append(
			obj.GetOwnerReferences(),
			metav1.OwnerReference{
				APIVersion: GroupVersion.String(),
				Kind:       "QuayRegistry",
				Name:       quay.GetName(),
				UID:        quay.GetUID(),
			},
		),
	)
	return obj
}

// RemoveOwnerReference removes the `ownerReference` of `QuayRegistry` on the given object.
func RemoveOwnerReference(quay *QuayRegistry, obj client.Object) (client.Object, error) {
	filteredOwnerReferences := []metav1.OwnerReference{}

	objectMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	for _, ownerRef := range objectMeta.GetOwnerReferences() {
		if ownerRef.Name != quay.GetName() {
			filteredOwnerReferences = append(filteredOwnerReferences, ownerRef)
		}
	}
	objectMeta.SetOwnerReferences(filteredOwnerReferences)

	return obj, nil
}

// ManagedKeysSecretNameFor returns the name of the `Secret` in which generated secret keys are stored.
func ManagedKeysSecretNameFor(quay *QuayRegistry) string {
	return strings.Join([]string{quay.GetName(), ManagedKeysName}, "-")
}

func IsManagedKeysSecretFor(quay *QuayRegistry, secret *corev1.Secret) bool {
	return strings.Contains(secret.GetName(), quay.GetName()+"-"+ManagedKeysName)
}

func IsManagedTLSSecretFor(quay *QuayRegistry, secret *corev1.Secret) bool {
	return strings.Contains(secret.GetName(), quay.GetName()+"-"+QuayConfigTLSSecretName)
}

// FieldGroupNameFor returns the field group name for a component kind.
func FieldGroupNameFor(cmp ComponentKind) (string, error) {
	switch cmp {
	case ComponentClair:
		return "SecurityScanner", nil
	case ComponentPostgres:
		return "Database", nil
	case ComponentClairPostgres:
		return "", nil
	case ComponentRedis:
		return "Redis", nil
	case ComponentObjectStorage:
		return "DistributedStorage", nil
	case ComponentRoute:
		return "HostSettings", nil
	case ComponentMirror:
		return "RepoMirror", nil
	case ComponentHPA:
		return "", nil
	case ComponentMonitoring:
		return "", nil
	case ComponentTLS:
		return "", nil
	case ComponentQuay:
		return "", nil
	default:
		return "", fmt.Errorf("unknown component: %q", cmp)
	}
}

// FieldGroupNamesForManagedComponents returns an slice of group names for all managed components.
func FieldGroupNamesForManagedComponents(quay *QuayRegistry) ([]string, error) {
	var fgns []string
	for _, cmp := range quay.Spec.Components {
		if !cmp.Managed {
			continue
		}

		fgn, err := FieldGroupNameFor(cmp.Kind)
		if err != nil {
			return nil, err
		}

		if len(fgn) == 0 {
			continue
		}

		fgns = append(fgns, fgn)
	}
	return fgns, nil
}

// ComponentSupportsOverride returns whether or not a given component supports the given override.
func ComponentSupportsOverride(component ComponentKind, override string) bool {
	var components []ComponentKind

	switch override {
	case "volumeSize":
		components = supportsVolumeOverride
	case "env":
		components = supportsEnvOverride
	case "replicas":
		components = supportsReplicasOverride
	case "affinity":
		components = supportsAffinityOverride
	}

	for _, cmp := range components {
		if component == cmp {
			return true
		}
	}
	return false
}

func init() {
	SchemeBuilder.Register(&QuayRegistry{}, &QuayRegistryList{})
}

// GetReplicasOverrideForComponent returns the overrides set by the user for the provided
// component. Returns nil if not set.
func GetReplicasOverrideForComponent(quay *QuayRegistry, kind ComponentKind) *int32 {
	for _, cmp := range quay.Spec.Components {
		if cmp.Kind != kind {
			continue
		}

		if cmp.Overrides == nil {
			return nil
		}

		return cmp.Overrides.Replicas
	}
	return nil
}

// GetVolumeSizeOverrideForComponent returns the volume size overrides set by the user for the
// provided component. Returns nil if not set.
func GetVolumeSizeOverrideForComponent(
	quay *QuayRegistry, kind ComponentKind,
) (qt *resource.Quantity) {
	for _, component := range quay.Spec.Components {
		if component.Kind != kind {
			continue
		}

		if component.Overrides != nil && component.Overrides.VolumeSize != nil {
			qt = component.Overrides.VolumeSize
		}
		return
	}
	return
}

// GetAffinityForComponent returns affinity overrides for the provided component
// if they are present, nil otherwise
func GetAffinityForComponent(quay *QuayRegistry, kind ComponentKind) (affinity *corev1.Affinity) {
	for _, cmp := range quay.Spec.Components {
		if cmp.Kind != kind {
			continue
		}

		if cmp.Overrides == nil {
			return
		}

		affinity = cmp.Overrides.Affinity
		return affinity
	}
	return
}

// GetEnvOverrideForComponent return the environment variables overrides for the provided
// component, nil is returned if not defined.
func GetEnvOverrideForComponent(quay *QuayRegistry, kind ComponentKind) []corev1.EnvVar {
	for _, cmp := range quay.Spec.Components {
		if cmp.Kind != kind {
			continue
		}

		if cmp.Overrides == nil {
			return nil
		}

		return cmp.Overrides.Env
	}
	return nil
}

// GetLabelsOverrideForComponent returns overriden labels for the provided component
// nil is returned if there are no label overrides
func GetLabelsOverrideForComponent(quay *QuayRegistry, kind ComponentKind) map[string]string {
	for _, cmp := range quay.Spec.Components {
		if cmp.Kind != kind {
			continue
		}

		if cmp.Overrides == nil {
			return nil
		}

		return cmp.Overrides.Labels
	}

	return nil
}

// ExceptionLabel checks if attempt to override label affects exceptional labels
func ExceptionLabel(override string) bool {
	for _, label := range []string{"quay-component", "app", "quay-operator/quayregistry"} {
		if override != label {
			continue
		}
		return true
	}
	return false
}

// GetAnnotationsOverrideForComponent returns overriden annotations for the provided component
// nil is returned if there are no annotation overrides
func GetAnnotationsOverrideForComponent(quay *QuayRegistry, kind ComponentKind) map[string]string {
	for _, cmp := range quay.Spec.Components {
		if cmp.Kind != kind {
			continue
		}

		if cmp.Overrides == nil {
			return nil
		}

		return cmp.Overrides.Annotations
	}

	return nil
}

// RemoveUnusedConditions is used to trim off conditions created by previous releases of this
// operator that are not used anymore.
func RemoveUnusedConditions(quay *QuayRegistry) {
	validConditionTypes := []ConditionType{
		ConditionTypeAvailable,
		ConditionTypeRolloutBlocked,
		ConditionComponentsCreated,
		ComponentQuayReady,
		ComponentPostgresReady,
		ComponentClairReady,
		ComponentClairPostgresReady,
		ComponentRedisReady,
		ComponentHPAReady,
		ComponentObjectStorageReady,
		ComponentRouteReady,
		ComponentMirrorReady,
		ComponentMonitoringReady,
		ComponentTLSReady,
	}

	newconds := []Condition{}
	for _, qcond := range quay.Status.Conditions {
		for _, valid := range validConditionTypes {
			if qcond.Type != valid {
				continue
			}
			newconds = append(newconds, qcond)
			break
		}
	}

	quay.Status.Conditions = newconds
}
