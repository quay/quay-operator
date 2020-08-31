package kustomize

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"

	"github.com/go-logr/logr"
	v1 "github.com/quay/quay-operator/api/v1"
)

const configSecretPrefix = "quay-config-secret"

func kustomizeDir() string {
	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(filename))

	return filepath.Join(path, "..", "..", "kustomize")
}

func appDir() string {
	return filepath.Join(kustomizeDir(), "tmp")
}

func overlayDir(desiredVersion v1.QuayVersion) string {
	return filepath.Join(kustomizeDir(), "overlays", "upstream", string(desiredVersion))
}

func upgradeOverlayDir(desiredVersion v1.QuayVersion) string {
	return filepath.Join(kustomizeDir(), "overlays", "upstream", string(desiredVersion), "upgrade")
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
func ModelFor(gvk schema.GroupVersionKind) k8sruntime.Object {
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
		return &appsv1.Deployment{}
	case schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Kind: "Role"}.String():
		return &rbac.Role{}
	case schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Kind: "RoleBinding"}.String():
		return &rbac.RoleBinding{}
	case schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "Route"}.String():
		return &routev1.Route{}
	case schema.GroupVersionKind{Group: "objectbucket.io", Version: "v1alpha1", Kind: "ObjectBucketClaim"}.String():
		return &objectbucket.ObjectBucketClaim{}
	default:
		panic(fmt.Sprintf("Missing model for GVK %s", gvk.String()))
	}
}

// generate uses Kustomize as a library to build the runtime objects to be applied to a cluster.
func generate(kustomization *types.Kustomization, overlay string, quayConfigFiles map[string][]byte) ([]k8sruntime.Object, error) {
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

	output := []k8sruntime.Object{}
	for _, resource := range resMap.Resources() {
		resourceJSON, err := resource.MarshalJSON()
		check(err)

		obj := ModelFor(schema.GroupVersionKind{
			Group:   resource.GetGvk().Group,
			Version: resource.GetGvk().Version,
			Kind:    resource.GetGvk().Kind,
		})

		if obj == nil {
			panic("TODO(alecmerdler): Not implemented for GroupVersionKind: " + resource.GetGvk().String())
		}

		err = json.Unmarshal(resourceJSON, obj)
		check(err)

		output = append(output, obj)
	}

	return output, nil
}

// KustomizationFor takes a `QuayRegistry` object and generates a Kustomization for it.
func KustomizationFor(quay *v1.QuayRegistry, quayConfigFiles map[string][]byte) (*types.Kustomization, error) {
	if quay == nil {
		return nil, errors.New("given QuayRegistry should not be nil")
	}

	configFiles := []string{}
	for key := range quayConfigFiles {
		configFiles = append(configFiles, filepath.Join("bundle", key))
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
	}

	componentPaths := []string{}
	managedFieldGroups := []string{}
	for _, component := range quay.Spec.Components {
		if component.Managed {
			componentPaths = append(componentPaths, filepath.Join("..", "components", component.Kind))
			managedFieldGroups = append(managedFieldGroups, fieldGroupFor(component.Kind))

			componentConfigFiles, err := componentConfigFilesFor(component.Kind, quay)
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

	return &types.Kustomization{
		TypeMeta: types.TypeMeta{
			APIVersion: types.KustomizationVersion,
			Kind:       types.KustomizationKind,
		},
		Namespace:       quay.GetNamespace(),
		NamePrefix:      quay.GetName() + "-",
		Resources:       []string{"../base"},
		Components:      componentPaths,
		SecretGenerator: generatedSecrets,
		CommonAnnotations: map[string]string{
			"quay-managed-fieldgroups": strings.Join(managedFieldGroups, ","),
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
	err := yaml.Unmarshal(configBundle.Data["config.yaml"], &flattenedConfig)
	check(err)

	isConfigField := func(field string) bool {
		return strings.Contains(field, ".config.yaml")
	}

	for key, file := range configBundle.Data {
		if isConfigField(key) {
			var valueYAML map[string]interface{}
			err = yaml.Unmarshal(file, &valueYAML)
			check(err)

			for configKey, configValue := range valueYAML {
				flattenedConfig[configKey] = configValue
			}
			delete(flattenedSecret.Data, key)
		}
	}

	flattenedConfigYAML, err := yaml.Marshal(flattenedConfig)
	check(err)

	flattenedSecret.Data["config.yaml"] = []byte(flattenedConfigYAML)

	return flattenedSecret, nil
}

// Inflate takes a `QuayRegistry` object and returns a set of Kubernetes objects representing a Quay deployment.
func Inflate(quay *v1.QuayRegistry, baseConfigBundle *corev1.Secret, secretKeysSecret *corev1.Secret, log logr.Logger) ([]k8sruntime.Object, error) {
	// Each `managedComponent` brings in their own generated `config.yaml` fields which are added to the base `Secret`
	componentConfigFiles := baseConfigBundle.DeepCopy().Data

	// Parse the user-provided config bundle.
	var parsedUserConfig map[string]interface{}
	err := yaml.Unmarshal(componentConfigFiles["config.yaml"], &parsedUserConfig)
	check(err)

	// Generate or pull out the SECRET_KEY and DATABASE_SECRET_KEY. Since these must be stable across
	// runs of the same config, we store them (and re-read them) from a specialized Secret.
	secretKey, databaseSecretKey, secretKeysSecret := handleSecretKeys(parsedUserConfig, secretKeysSecret, quay, log)

	componentConfigFiles["quay.config.yaml"] = encode(map[string]interface{}{
		"SETUP_COMPLETE":      true,
		"DATABASE_SECRET_KEY": databaseSecretKey,
		"SECRET_KEY":          secretKey,
	})

	for _, component := range quay.Spec.Components {
		if component.Managed {
			configFile, err := ConfigFileFor(component.Kind, quay)
			check(err)
			componentConfigFiles[component.Kind+".config.yaml"] = configFile
		}
	}

	kustomization, err := KustomizationFor(quay, componentConfigFiles)
	check(err)

	var overlay string
	if quay.Spec.DesiredVersion == quay.Status.CurrentVersion {
		overlay = overlayDir(quay.Spec.DesiredVersion)
	} else {
		overlay = upgradeOverlayDir(quay.Spec.DesiredVersion)
	}
	resources, err := generate(kustomization, overlay, componentConfigFiles)
	check(err)

	for index, resource := range resources {
		_ = reflect.ValueOf(resource).Type()
		objectMeta, err := meta.Accessor(resource)
		check(err)

		if strings.Contains(objectMeta.GetName(), configSecretPrefix+"-") {
			configBundleSecret, err := flattenSecret(resource.(*corev1.Secret))
			check(err)

			resources[index] = configBundleSecret
		}
	}

	secretKeysSecret.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})
	resources = append(resources, secretKeysSecret)

	for _, resource := range resources {
		objectMeta, err := meta.Accessor(resource)
		check(err)

		objectMeta.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "QuayRegistry",
				Name:       quay.GetName(),
				UID:        quay.GetUID(),
			},
		})
	}

	return resources, err
}
