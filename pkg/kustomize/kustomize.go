package kustomize

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-logr/logr"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	route "github.com/openshift/api/route/v1"
	apps "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
)

const (
	QuayRegistryNameLabel = "quay-operator/quayregistry"

	configSecretPrefix                = "quay-config-secret"
	registryHostnameAnnotation        = "quay-registry-hostname"
	buildManagerHostnameAnnotation    = "quay-buildmanager-hostname"
	managedFieldGroupsAnnotation      = "quay-managed-fieldgroups"
	operatorServiceEndpointAnnotation = "quay-operator-service-endpoint"

	podNamespaceKey = "MY_POD_NAMESPACE"

	componentImagePrefix = "RELATED_IMAGE_COMPONENT_"

	configEditorUser = "quayconfig"
)

// componentImageFor checks for an environment variable indicating which component container image
// to use. If set, returns a Kustomize image override for the given component.
func componentImageFor(component string) types.Image {
	envVarFor := map[string]string{
		"base":     componentImagePrefix + "QUAY",
		"clair":    componentImagePrefix + "CLAIR",
		"redis":    componentImagePrefix + "REDIS",
		"postgres": componentImagePrefix + "POSTGRES",
	}
	defaultImagesFor := map[string]string{
		"base":     "quay.io/projectquay/quay",
		"clair":    "quay.io/projectquay/clair",
		"redis":    "centos/redis-32-centos7",
		"postgres": "centos/postgresql-10-centos7",
	}

	imageOverride := types.Image{
		Name: defaultImagesFor[component],
	}
	image := os.Getenv(envVarFor[component])
	if image != "" {
		if len(strings.Split(image, "@")) != 2 {
			panic(envVarFor[component] + " must use manifest digest reference")
		}

		imageOverride.NewName = strings.Split(image, "@")[0]
		imageOverride.Digest = strings.Split(image, "@")[1]
	}

	return imageOverride
}

func kustomizeDir() string {
	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(filename))

	return filepath.Join(path, "..", "..", "kustomize")
}

func appDir() string {
	return filepath.Join(kustomizeDir(), "tmp")
}

func overlayDir() string {
	return filepath.Join(kustomizeDir(), "overlays", "current")
}

func upgradeOverlayDir() string {
	return filepath.Join(kustomizeDir(), "overlays", "current", "upgrade")
}

func configEditorOnlyOverlay() string {
	return filepath.Join(kustomizeDir(), "overlays", "current", "config-only")
}

func rolloutBlocked(quay *v1.QuayRegistry) bool {
	if cond := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeRolloutBlocked); cond != nil && cond.Status == metav1.ConditionTrue {
		return true
	}
	return false
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)

	return yamlified
}

func decode(bytes []byte) interface{} {
	var value interface{}
	_ = yaml.Unmarshal(bytes, &value)

	return value
}

// ModelFor returns an empty Kubernetes object instance for the given `GroupVersionKind`.
// Example: Calling with `core.v1.Secret` GVK returns an empty `corev1.Secret` instance.
func ModelFor(gvk schema.GroupVersionKind) client.Object {
	switch gvk.String() {
	case schema.GroupVersionKind{Version: "v1", Kind: "Secret"}.String():
		return &corev1.Secret{}
	case schema.GroupVersionKind{Version: "v1", Kind: "Service"}.String():
		return &corev1.Service{}
	case schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}.String():
		return &corev1.ConfigMap{}
	case schema.GroupVersionKind{Version: "v1", Kind: "PersistentVolumeClaim"}.String():
		return &corev1.PersistentVolumeClaim{}
	case schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}.String():
		return &apps.Deployment{}
	case schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Kind: "Role"}.String():
		return &rbac.Role{}
	case schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Kind: "RoleBinding"}.String():
		return &rbac.RoleBinding{}
	case schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "Route"}.String():
		return &route.Route{}
	case schema.GroupVersionKind{Group: "objectbucket.io", Version: "v1alpha1", Kind: "ObjectBucketClaim"}.String():
		return &objectbucket.ObjectBucketClaim{}
	case schema.GroupVersionKind{Group: "autoscaling", Version: "v2beta2", Kind: "HorizontalPodAutoscaler"}.String():
		return &autoscaling.HorizontalPodAutoscaler{}
	case schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}.String():
		return &batchv1.Job{}
	default:
		panic(fmt.Sprintf("Missing model for GVK %s", gvk.String()))
	}
}

// generate uses Kustomize as a library to build the runtime objects to be applied to a cluster.
func generate(kustomization *types.Kustomization, overlay string, quayConfigFiles map[string][]byte) ([]client.Object, error) {
	fSys := filesys.MakeEmptyDirInMemory()
	err := filepath.Walk(kustomizeDir(), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			err = fSys.WriteFile(path, f)
			if err != nil {
				return err
			}
		}
		return nil
	})
	check(err)

	// Write `kustomization.yaml` to filesystem
	kustomizationFile, err := yaml.Marshal(kustomization)
	check(err)
	err = fSys.WriteFile(filepath.Join(appDir(), "kustomization.yaml"), kustomizationFile)
	check(err)

	// Add all Quay config files to directory to be included in the generated `Secret`
	for fileName, file := range quayConfigFiles {
		check(err)
		err = fSys.WriteFile(filepath.Join(appDir(), "bundle", fileName), file)
		check(err)
	}

	opts := &krusty.Options{}
	k := krusty.MakeKustomizer(fSys, opts)
	resMap, err := k.Run(overlay)
	check(err)

	output := []client.Object{}
	for _, resource := range resMap.Resources() {
		resourceJSON, err := resource.MarshalJSON()
		check(err)

		obj := ModelFor(schema.GroupVersionKind{
			Group:   resource.GetGvk().Group,
			Version: resource.GetGvk().Version,
			Kind:    resource.GetGvk().Kind,
		})

		if obj == nil {
			panic("TODO: Not implemented for GroupVersionKind: " + resource.GetGvk().String())
		}

		err = json.Unmarshal(resourceJSON, obj)
		check(err)

		output = append(output, obj)
	}

	return output, nil
}

// KustomizationFor takes a `QuayRegistry` object and generates a Kustomization for it.
func KustomizationFor(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, quayConfigFiles map[string][]byte) (*types.Kustomization, error) {
	if quay == nil {
		return nil, errors.New("given QuayRegistry should not be nil")
	}

	configFiles := []string{}
	for key := range quayConfigFiles {
		configFiles = append(configFiles, filepath.Join("bundle", key))
	}

	configEditorPassword, err := generateRandomString(16)
	if err != nil {
		return nil, err
	}

	generatedSecrets := []types.SecretArgs{
		{
			GeneratorArgs: types.GeneratorArgs{
				Name: configSecretPrefix,
				KvPairSources: types.KvPairSources{
					FileSources: configFiles,
				},
			},
		},
		{
			GeneratorArgs: types.GeneratorArgs{
				Name: v1.ManagedKeysName,
				KvPairSources: types.KvPairSources{
					LiteralSources: []string{
						"DATABASE_SECRET_KEY=" + ctx.DatabaseSecretKey,
						"SECRET_KEY=" + ctx.SecretKey,
					},
				},
			},
		},
		{
			GeneratorArgs: types.GeneratorArgs{
				Name: "quay-config-editor-credentials",
				KvPairSources: types.KvPairSources{
					LiteralSources: []string{
						"username=" + configEditorUser,
						"password=" + configEditorPassword,
					},
				},
			},
		},
	}

	componentPaths := []string{}
	managedFieldGroups := []string{}
	for _, component := range quay.Spec.Components {
		if component.Managed {
			componentPaths = append(componentPaths, filepath.Join("..", "components", component.Kind))
			managedFieldGroups = append(managedFieldGroups, fieldGroupNameFor(component.Kind))

			componentConfigFiles, err := componentConfigFilesFor(component.Kind, quay, quayConfigFiles)
			if componentConfigFiles == nil || err != nil {
				continue
			}

			sources := []string{}
			for filename, fileValue := range componentConfigFiles {
				sources = append(sources, strings.Join([]string{filename, string(fileValue)}, "="))
			}

			generatedSecrets = append(generatedSecrets, types.SecretArgs{
				GeneratorArgs: types.GeneratorArgs{
					Name:     component.Kind + "-config-secret",
					Behavior: "merge",
					KvPairSources: types.KvPairSources{
						LiteralSources: sources,
					},
				},
			})
		}
	}

	images := []types.Image{}
	for _, component := range append(quay.Spec.Components, v1.Component{Kind: "base", Managed: true}) {
		if component.Managed {
			imageOverride := componentImageFor(component.Kind)
			if imageOverride.Digest != "" {
				images = append(images, imageOverride)
			}
		}
	}

	return &types.Kustomization{
		TypeMeta: types.TypeMeta{
			APIVersion: types.KustomizationVersion,
			Kind:       types.KustomizationKind,
		},
		Namespace:       quay.GetNamespace(),
		NamePrefix:      quay.GetName() + "-",
		Resources:       []string{"../base"},
		Images:          images,
		Components:      componentPaths,
		SecretGenerator: generatedSecrets,
		CommonLabels: map[string]string{
			QuayRegistryNameLabel: quay.GetName(),
		},
		CommonAnnotations: map[string]string{
			managedFieldGroupsAnnotation:      strings.ReplaceAll(strings.Join(managedFieldGroups, ","), ",,", ","),
			registryHostnameAnnotation:        ctx.ServerHostname,
			buildManagerHostnameAnnotation:    strings.Split(ctx.BuildManagerHostname, ":")[0],
			operatorServiceEndpointAnnotation: operatorServiceEndpoint(),
		},
		// NOTE: Using `vars` in Kustomize is kinda ugly because it's basically templating, so don't abuse them
		Vars: []types.Var{
			{
				Name: "QE_K8S_CONFIG_SECRET",
				ObjRef: types.Target{
					APIVersion: "v1",
					Gvk: resid.Gvk{
						Kind: "Secret",
					},
					Name: "quay-config-secret",
				},
			},
		},
	}, nil
}

// flattenSecret takes all Quay config fields in given secret and combines them under `config.yaml` key.
func flattenSecret(configBundle *corev1.Secret) (*corev1.Secret, error) {
	flattenedSecret := configBundle.DeepCopy()

	var flattenedConfig map[string]interface{}
	if err := yaml.Unmarshal(configBundle.Data["config.yaml"], &flattenedConfig); err != nil {
		return nil, err
	}

	isConfigField := func(field string) bool {
		return strings.Contains(field, ".config.yaml")
	}

	for key, file := range configBundle.Data {
		if isConfigField(key) {
			var valueYAML map[string]interface{}
			if err := yaml.Unmarshal(file, &valueYAML); err != nil {
				return nil, err
			}

			for configKey, configValue := range valueYAML {
				flattenedConfig[configKey] = configValue
			}
			delete(flattenedSecret.Data, key)
		}
	}

	flattenedConfigYAML, err := yaml.Marshal(flattenedConfig)
	if err != nil {
		return nil, err
	}

	flattenedSecret.Data["config.yaml"] = []byte(flattenedConfigYAML)

	return flattenedSecret, nil
}

// Inflate takes a `QuayRegistry` object and returns a set of Kubernetes objects representing a Quay deployment.
func Inflate(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, baseConfigBundle *corev1.Secret, log logr.Logger) ([]client.Object, error) {
	// Each managed component brings its own generated `config.yaml` fields
	// which are accumulated under the key `<component>.config.yaml` and then added to the base `Secret`.
	componentConfigFiles := baseConfigBundle.DeepCopy().Data

	var parsedUserConfig map[string]interface{}
	if err := yaml.Unmarshal(baseConfigBundle.Data["config.yaml"], &parsedUserConfig); err != nil {
		return nil, err
	}

	// Generate or pull out the `SECRET_KEY` and `DATABASE_SECRET_KEY`. Since these must be stable across
	// runs of the same config, we store them (and re-read them) from a specialized `Secret`.
	if ctx.DatabaseSecretKey == "" {
		if key, found := parsedUserConfig["DATABASE_SECRET_KEY"]; found {
			log.Info("`DATABASE_SECRET_KEY` found in user-provided config")

			ctx.DatabaseSecretKey = key.(string)
		} else {
			log.Info("`DATABASE_SECRET_KEY` not found in user-provided config, generating new one")

			databaseSecretKey, err := generateRandomString(secretKeyLength)
			if err != nil {
				return nil, err
			}

			ctx.DatabaseSecretKey = databaseSecretKey
		}
	}

	if ctx.SecretKey == "" {
		if key, found := parsedUserConfig["SECRET_KEY"]; found {
			log.Info("`SECRET_KEY` found in user-provided config")

			ctx.SecretKey = key.(string)
		} else {
			log.Info("`SECRET_KEY` not found in user-provided config, generating new one")

			secretKey, err := generateRandomString(secretKeyLength)
			if err != nil {
				return nil, err
			}

			ctx.SecretKey = secretKey
		}
	}

	parsedUserConfig["SETUP_COMPLETE"] = true
	parsedUserConfig["DATABASE_SECRET_KEY"] = ctx.DatabaseSecretKey
	parsedUserConfig["SECRET_KEY"] = ctx.SecretKey

	for field, value := range BaseConfig() {
		if _, ok := parsedUserConfig[field]; !ok {
			parsedUserConfig[field] = value
		}
	}
	componentConfigFiles["config.yaml"] = encode(parsedUserConfig)

	for _, component := range quay.Spec.Components {
		if component.Managed {
			fieldGroup, err := FieldGroupFor(ctx, component.Kind, quay)
			if err != nil {
				return nil, err
			}

			componentConfigFiles[component.Kind+".config.yaml"] = encode(fieldGroup)
		}
	}

	log.Info("Ensuring `ssl.cert` and `ssl.key` pair for Quay app TLS")
	tlsCert, tlsKey, err := EnsureTLSFor(ctx, quay, baseConfigBundle.Data["ssl.cert"], baseConfigBundle.Data["ssl.key"])
	if err != nil {
		return nil, err
	}

	componentConfigFiles["ssl.cert"] = tlsCert
	componentConfigFiles["ssl.key"] = tlsKey

	kustomization, err := KustomizationFor(ctx, quay, componentConfigFiles)
	check(err)

	var overlay string
	if rolloutBlocked(quay) {
		overlay = configEditorOnlyOverlay()
	} else if quay.Status.CurrentVersion == "" {
		overlay = upgradeOverlayDir()
	} else {
		overlay = overlayDir()
	}
	resources, err := generate(kustomization, overlay, componentConfigFiles)
	check(err)

	for index, resource := range resources {
		objectMeta, err := meta.Accessor(resource)
		check(err)

		if strings.Contains(objectMeta.GetName(), configSecretPrefix+"-") {
			configBundleSecret, err := flattenSecret(resource.(*corev1.Secret))
			check(err)

			resources[index] = configBundleSecret
		}
	}

	for _, resource := range resources {
		resource, err = v1.EnsureOwnerReference(quay, resource)
		check(err)
	}

	return resources, err
}

func operatorServiceEndpoint() string {
	// For local development, use ngrok or some other tool to expose local server to config editor.
	if devEndpoint := os.Getenv("DEV_OPERATOR_ENDPOINT"); devEndpoint != "" {
		return devEndpoint
	}

	endpoint := "http://quay-operator"
	if ns := os.Getenv(podNamespaceKey); ns != "" {
		endpoint = strings.Join([]string{endpoint, ns}, ".")
	}

	return strings.Join([]string{endpoint, "7071"}, ":")
}
