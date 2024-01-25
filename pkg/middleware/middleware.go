package middleware

import (
	"fmt"
	"strings"

	route "github.com/openshift/api/route/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
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
func Process(quay *v1.QuayRegistry, qctx *quaycontext.QuayRegistryContext, obj client.Object, skipres bool) (client.Object, error) {
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
		delete(configBundleSecret.Data, "clair-ssl.key")
		delete(configBundleSecret.Data, "clair-ssl.crt")

		// Remove the ca certs since they are being mounted by extra-ca-certs
		for key := range configBundleSecret.Data {
			if strings.HasPrefix(key, "extra_ca_cert_") {
				delete(configBundleSecret.Data, key)
			}
		}

		return configBundleSecret, nil
	}

	// we need to remove
	// all unused annotations from postgres deployment to avoid its redeployment.
	if dep, ok := obj.(*appsv1.Deployment); ok {
		// override any environment variable being provided through the component
		// Override.Env property. XXX this does not include InitContainers.
		component := labels.Set(objectMeta.GetAnnotations()).Get("quay-component")
		kind := v1.ComponentKind(component)
		if v1.ComponentIsManaged(quay.Spec.Components, kind) {
			for _, oenv := range v1.GetEnvOverrideForComponent(quay, kind) {
				for i := range dep.Spec.Template.Spec.Containers {
					ref := &dep.Spec.Template.Spec.Containers[i]
					UpsertContainerEnv(ref, oenv)
				}
			}

			// Add additional default environment variables to Quay deployment
			if kind == v1.ComponentQuay {
				oenv := corev1.EnvVar{Name: "QUAY_VERSION", Value: string(v1.QuayVersionCurrent)}
				for i := range dep.Spec.Template.Spec.Containers {
					ref := &dep.Spec.Template.Spec.Containers[i]
					UpsertContainerEnv(ref, oenv)
				}
			}
		}

		if oaff := v1.GetAffinityForComponent(quay, kind); oaff != nil {
			dep.Spec.Template.Spec.Affinity = oaff
		}

		// Add annotations to track the hash of the cluster service CA. This is to ensure that we redeploy when the cluster service CA changes.
		dep.Annotations[v1.ClusterServiceCAName] = qctx.ClusterServiceCAHash
		dep.Annotations[v1.ClusterTrustedCAName] = qctx.ClusterTrustedCAHash
		dep.Spec.Template.Annotations[v1.ClusterServiceCAName] = qctx.ClusterServiceCAHash
		dep.Spec.Template.Annotations[v1.ClusterTrustedCAName] = qctx.ClusterTrustedCAHash

		// here we do an attempt to setting the default or overwriten number of replicas
		// for clair, quay and mirror. we can't do that if horizontal pod autoscaler is
		// in managed state as we would be stomping in the values defined by the hpa
		// controller.
		if !v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentHPA) {
			for kind, depsuffix := range map[v1.ComponentKind]string{
				v1.ComponentClair:  "clair-app",
				v1.ComponentMirror: "quay-mirror",
				v1.ComponentQuay:   "quay-app",
			} {
				if !strings.HasSuffix(dep.Name, depsuffix) {
					continue
				}

				// if the number of replicas has been set through kustomization
				// we do not override it, just break here and move on with it.
				// we have to adopt this approach because during "upgrades" the
				// number of replicas is set to zero and we don't want to stomp
				// it.
				if dep.Spec.Replicas != nil {
					break
				}

				// if no number of replicas has been set in kustomization files
				// we set its value to two or to the value provided by the user
				// as an override (if provided).
				desired := ptr.To[int32](2)
				if r := v1.GetReplicasOverrideForComponent(quay, kind); r != nil {
					desired = r
				}

				dep.Spec.Replicas = desired
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

		if olabels := v1.GetLabelsOverrideForComponent(quay, kind); olabels != nil {
			if dep.Labels == nil {
				dep.Labels = map[string]string{}
			}
			if dep.Spec.Template.Labels == nil {
				dep.Spec.Template.Labels = map[string]string{}
			}
			for key, value := range olabels {
				if v1.ExceptionLabel(key) {
					continue
				}
				dep.Labels[key] = value
				dep.Spec.Template.Labels[key] = value
			}
		}

		if oannot := v1.GetAnnotationsOverrideForComponent(quay, kind); oannot != nil {
			if dep.Annotations == nil {
				dep.Annotations = map[string]string{}
			}
			if dep.Spec.Template.Annotations == nil {
				dep.Spec.Template.Annotations = map[string]string{}
			}
			for key, value := range oannot {
				dep.Annotations[key] = value
				dep.Spec.Template.Annotations[key] = value
			}
		}

		// TODO we must not set annotations in objects where they are not needed. we also
		// must stop mangling objects in this "middleware" thingy, what is the point of
		// using kustomize if we keep changing stuff on the fly ?
		isQuayDB := strings.Contains(dep.GetName(), "quay-database")
		isClairDB := strings.Contains(dep.GetName(), "clair-postgres")
		if isQuayDB || isClairDB {
			delete(dep.Spec.Template.Annotations, "quay-registry-hostname")
			delete(dep.Spec.Template.Annotations, "quay-buildmanager-hostname")
			delete(dep.Spec.Template.Annotations, "quay-operator-service-endpoint")
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
			override = v1.GetVolumeSizeOverrideForComponent(quay, v1.ComponentPostgres)
		case "clair-postgres":
			override = v1.GetVolumeSizeOverrideForComponent(quay, v1.ComponentClair)
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

	if _, ok := obj.(*autoscalingv1.HorizontalPodAutoscaler); ok {
		componentMap := map[string]v1.ComponentKind{
			"mirror": v1.ComponentMirror,
			"clair":  v1.ComponentClair,
		}
		// If the HPA is not for a managed component, return nil
		if component, ok := componentMap[quayComponentLabel]; ok {
			if !v1.ComponentIsManaged(quay.Spec.Components, component) {
				return nil, nil
			}
		}
		return obj, nil
	}

	if job, ok := obj.(*batchv1.Job); ok {
		for _, oenv := range v1.GetEnvOverrideForComponent(quay, v1.ComponentQuay) {
			for i := range job.Spec.Template.Spec.Containers {
				ref := &job.Spec.Template.Spec.Containers[i]
				UpsertContainerEnv(ref, oenv)
			}
		}
		// if we are deploying without resource requests we have to remove them
		if skipres {
			var noresources corev1.ResourceRequirements
			for i := range job.Spec.Template.Spec.Containers {
				job.Spec.Template.Spec.Containers[i].Resources = noresources
			}
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
