package kustomize

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"

	v1 "github.com/quay/quay-operator/api/v1"
)

const kustomizeDir = "../../kustomize"

var appDir = path.Join(kustomizeDir, "tmp")

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// ModelFor returns an empty Kubernetes object instance for the given `GroupVersionKind`.
// Example: Calling with `core.v1.Secret` GVK returns an empty `corev1.Secret` instance.
func ModelFor(gvk schema.GroupVersionKind) runtime.Object {
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
	default:
		return nil
	}
}

// generate uses Kustomize as a library to build the runtime objects to be applied to a cluster.
func generate(kustomization *types.Kustomization, quayConfigFiles map[string][]byte) ([]runtime.Object, error) {
	fSys := filesys.MakeFsOnDisk()

	err := fSys.RemoveAll(path.Join(appDir))
	check(err)
	err = fSys.MkdirAll(path.Join(appDir, "bundle"))
	check(err)

	// Write `kustomization.yaml` to filesystem
	kustomizationFile, err := yaml.Marshal(kustomization)
	check(err)
	err = fSys.WriteFile(path.Join(appDir, "kustomization.yaml"), kustomizationFile)
	check(err)

	// Add all Quay config files to directory to be included in the generated `Secret`
	check(err)
	for fileName, file := range quayConfigFiles {
		check(err)
		err = fSys.WriteFile(path.Join(appDir, "bundle", fileName), file)
		check(err)
	}

	opts := &krusty.Options{}
	k := krusty.MakeKustomizer(fSys, opts)
	resMap, err := k.Run(appDir)
	check(err)

	output := []runtime.Object{}
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

func KustomizationFor(quay *v1.QuayRegistry, baseConfigBundle *corev1.Secret) (*types.Kustomization, error) {
	if quay == nil {
		return nil, errors.New("given QuayRegistry should not be nil")
	}

	components := []string{}
	for _, managedComponent := range quay.Spec.ManagedComponents {
		components = append(components, path.Join("..", "components", managedComponent.Kind))
	}
	configFiles := []string{}
	for key := range baseConfigBundle.Data {
		configFiles = append(configFiles, path.Join("bundle", key))
	}

	return &types.Kustomization{
		TypeMeta: types.TypeMeta{
			APIVersion: types.KustomizationVersion,
			Kind:       types.KustomizationKind,
		},
		Resources:  []string{"../base"},
		Components: components,
		SecretGenerator: []types.SecretArgs{
			{
				GeneratorArgs: types.GeneratorArgs{
					Name:     "quay-config-secret",
					Behavior: "merge",
					KvPairSources: types.KvPairSources{
						FileSources: configFiles,
					},
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
		return !strings.Contains(field, ".")
	}

	for key, value := range configBundle.Data {
		if isConfigField(key) {
			var valueYAML interface{}
			err = yaml.Unmarshal(value, &valueYAML)
			check(err)

			flattenedConfig[key] = valueYAML
			delete(flattenedSecret.Data, key)
		}
	}

	flattenedConfigYAML, err := yaml.Marshal(flattenedConfig)
	check(err)

	flattenedSecret.Data["config.yaml"] = []byte(flattenedConfigYAML)

	return flattenedSecret, nil
}

// Inflate takes a `QuayRegistry` object and returns a set of Kubernetes objects representing a Quay deployment.
func Inflate(quay *v1.QuayRegistry, baseConfigBundle *corev1.Secret) ([]runtime.Object, error) {
	kustomization, err := KustomizationFor(quay, baseConfigBundle)
	check(err)

	resources, err := generate(kustomization, baseConfigBundle.Data)
	check(err)

	for index, resource := range resources {
		k8sType := reflect.ValueOf(resource).Type()
		objectMeta, err := meta.Accessor(resource)
		check(err)

		// FIXME(alecmerdler): Debugging
		fmt.Println(k8sType, objectMeta.GetName())

		if strings.Contains(objectMeta.GetName(), "quay-config-secret-") {
			configBundleSecret, err := flattenSecret(resource.(*corev1.Secret))
			check(err)

			resources[index] = configBundleSecret
		}
	}

	return resources, err
}
