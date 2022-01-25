package middleware

import (
	"fmt"
	"strings"

	route "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
)

const (
	configSecretPrefix    = "quay-config-secret"
	fieldGroupsAnnotation = "quay-managed-fieldgroups"
)

// Process applies any additional middleware steps to a managed k8s object that cannot be
// accomplished using the Kustomize toolchain. if skipres is set all resource requests are
// trimmed from the objects thus deploying quay with a much smaller footprint.
func Process(quay *v1.QuayRegistry, obj client.Object, skipres bool) (client.Object, error) {
	objectMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	quayComponentLabel := labels.Set(objectMeta.GetLabels()).Get("quay-component")

	// Flatten config bundle `Secret`
	if strings.Contains(objectMeta.GetName(), configSecretPrefix+"-") {
		configBundleSecret, err := FlattenSecret(obj.(*corev1.Secret))
		if err != nil {
			return nil, err
		}

		// NOTE: Remove user-provided TLS cert/key pair so it is not mounted to
		// `/conf/stack`, otherwise it will affect Quay's generated NGINX config
		delete(configBundleSecret.Data, "ssl.cert")
		delete(configBundleSecret.Data, "ssl.key")

		return configBundleSecret, nil
	}

	// we have to set a special annotation in the config editor deployment, this annotation is
	// then used as an environment variable and consumed by the app. we also need to remove
	// all unused annotations from postgres deployment to avoid its redeployment.
	if dep, ok := obj.(*appsv1.Deployment); ok {

		// override any environment variable being provided through the component
		// Override.Env property. XXX this does not include InitContainers.
		component := labels.Set(objectMeta.GetAnnotations()).Get("quay-component")
		kind := v1.ComponentKind(component)
		if v1.ComponentIsManaged(quay.Spec.Components, kind) {
			for _, oenv := range getEnvOverrideForComponent(quay, kind) {
				for i := range dep.Spec.Template.Spec.Containers {
					ref := &dep.Spec.Template.Spec.Containers[i]
					UpsertContainerEnv(ref, oenv)
				}
			}
		}

		// if HPA is unmanaged we set the number of replicas to 2 for some of our
		// components (clair, mirror and quay). TODO(ricardomaraschini): this should
		// be an override so users can determine the desired number of replicas.
		noreplicas := dep.Spec.Replicas == nil
		if noreplicas && !v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentHPA) {
			for _, suffix := range []string{"clair-app", "quay-mirror", "quay-app"} {
				if !strings.HasSuffix(dep.Name, suffix) {
					continue
				}

				replicas := int32(2)
				dep.Spec.Replicas = &replicas
				break
			}
		}

		if skipres {
			// if we are deploying without resource requests we have to remove them
			var noresources corev1.ResourceRequirements
			for i := range dep.Spec.Template.Spec.Containers {
				dep.Spec.Template.Spec.Containers[i].Resources = noresources
			}
		}

		// TODO we must not set annotations in objects where they are not needed. we also
		// must stop mangling objects in this "middleware" thingy, what is the point of
		// using kustomize if we keep changing stuff on the fly ?
		if strings.Contains(dep.GetName(), "quay-database") {
			delete(dep.Spec.Template.Annotations, "quay-registry-hostname")
			delete(dep.Spec.Template.Annotations, "quay-buildmanager-hostname")
			delete(dep.Spec.Template.Annotations, "quay-operator-service-endpoint")
			return dep, nil
		}

		if !strings.Contains(dep.GetName(), "quay-config-editor") {
			return dep, nil
		}

		fgns, err := v1.FieldGroupNamesForManagedComponents(quay)
		if err != nil {
			return nil, err
		}

		dep.Spec.Template.Annotations[fieldGroupsAnnotation] = strings.Join(fgns, ",")
		return dep, nil
	}

	// If the current object is a PVC, check for volume override
	if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
		var override *resource.Quantity
		switch quayComponentLabel {
		case "postgres":
			override = getVolumeSizeOverrideForComponent(quay, v1.ComponentPostgres)
		case "clair-postgres":
			override = getVolumeSizeOverrideForComponent(quay, v1.ComponentClair)
		}

		// If override was not provided
		if override == nil {
			return pvc, nil
		}

		// Ensure that volume size is not being reduced
		pvcstorage := pvc.Spec.Resources.Requests.Storage()
		if pvcstorage != nil && override.Cmp(*pvcstorage) == -1 {
			return nil, fmt.Errorf(
				"cannot shrink volume override size from %s to %s",
				pvcstorage.String(),
				override.String(),
			)
		}

		pvc.Spec.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: *override,
		}

		return pvc, nil
	}

	if job, ok := obj.(*batchv1.Job); ok && skipres {
		// if we are deploying without resource requests we have to remove them
		var noresources corev1.ResourceRequirements
		for i := range job.Spec.Template.Spec.Containers {
			job.Spec.Template.Spec.Containers[i].Resources = noresources
		}
	}

	rt, ok := obj.(*route.Route)
	if !ok {
		return obj, nil
	}

	// if we are managing TLS we can simply return the original route as no change is needed.
	if v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentTLS) {
		return obj, nil
	}

	if quayComponentLabel != "quay-app-route" && quayComponentLabel != "quay-builder-route" {
		return obj, nil
	}

	// if we are not managing TLS then user has provided its own key pair. on this case we
	// set up the route as passthrough thus delegating TLS termination to the pods. as quay
	// builders route uses GRPC we do not change its route's target port.
	rt.Spec.TLS = &route.TLSConfig{
		Termination:                   route.TLSTerminationPassthrough,
		InsecureEdgeTerminationPolicy: route.InsecureEdgeTerminationPolicyRedirect,
	}
	if quayComponentLabel == "quay-app-route" {
		rt.Spec.Port = &route.RoutePort{
			TargetPort: intstr.Parse("https"),
		}
	}
	return rt, nil
}

// UpsertContainerEnv updates or inserts an environment variable into provided container.
func UpsertContainerEnv(container *corev1.Container, newv corev1.EnvVar) {
	for i, origv := range container.Env {
		if origv.Name != newv.Name {
			continue
		}

		container.Env[i] = newv
		return
	}

	// if not found then append it to the end.
	container.Env = append(container.Env, newv)
}

// FlattenSecret takes all Quay config fields in given secret and combines them under
// `config.yaml` key.
func FlattenSecret(configBundle *corev1.Secret) (*corev1.Secret, error) {
	flattenedSecret := configBundle.DeepCopy()

	var flattenedConfig map[string]interface{}
	if err := yaml.Unmarshal(configBundle.Data["config.yaml"], &flattenedConfig); err != nil {
		return nil, err
	}

	for key, file := range configBundle.Data {
		isConfig := strings.Contains(key, ".config.yaml")
		if !isConfig {
			continue
		}

		var valueYAML map[string]interface{}
		if err := yaml.Unmarshal(file, &valueYAML); err != nil {
			return nil, err
		}

		for configKey, configValue := range valueYAML {
			flattenedConfig[configKey] = configValue
		}

		delete(flattenedSecret.Data, key)
	}

	flattenedConfigYAML, err := yaml.Marshal(flattenedConfig)
	if err != nil {
		return nil, err
	}

	flattenedSecret.Data["config.yaml"] = flattenedConfigYAML
	return flattenedSecret, nil
}

// getEnvOverrideForComponent return the environment variables overrides for the provided
// component, nil is returned if not defined. Each component has its own env override, but
// not Base. Base is not consider an ordinary component so its env overides are kept in the
// root of the spec object.
func getEnvOverrideForComponent(quay *v1.QuayRegistry, kind v1.ComponentKind) []corev1.EnvVar {
	if kind == v1.ComponentBase {
		if quay.Spec.Overrides == nil {
			return nil
		}
		return quay.Spec.Overrides.Env
	}

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

func getVolumeSizeOverrideForComponent(
	quay *v1.QuayRegistry, componentKind v1.ComponentKind,
) (volumeSizeOverride *resource.Quantity) {
	for _, component := range quay.Spec.Components {
		if component.Kind != componentKind {
			continue
		}

		if component.Overrides != nil && component.Overrides.VolumeSize != nil {
			volumeSizeOverride = component.Overrides.VolumeSize
		}
		return
	}
	return
}
