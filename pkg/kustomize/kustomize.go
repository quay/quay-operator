package kustomize

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	route "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apps "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/yaml"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/middleware"
)

const (
	QuayRegistryNameLabel = "quay-operator/quayregistry"

	configSecretPrefix                = "quay-config-secret"
	registryHostnameAnnotation        = "quay-registry-hostname"
	buildManagerHostnameAnnotation    = "quay-buildmanager-hostname"
	operatorServiceEndpointAnnotation = "quay-operator-service-endpoint"

	podNamespaceKey = "MY_POD_NAMESPACE"

	componentImagePrefix = "RELATED_IMAGE_COMPONENT_"

	configEditorUser = "quayconfig"
)

// componentImageFor checks for an environment variable indicating which component container image
// to use. If set, returns a Kustomize image override for the given component.
func componentImageFor(component v1.ComponentKind) (types.Image, error) {
	envVarFor := map[v1.ComponentKind]string{
		v1.ComponentBase:     componentImagePrefix + "QUAY",
		v1.ComponentClair:    componentImagePrefix + "CLAIR",
		v1.ComponentRedis:    componentImagePrefix + "REDIS",
		v1.ComponentPostgres: componentImagePrefix + "POSTGRES",
	}
	defaultImagesFor := map[v1.ComponentKind]string{
		v1.ComponentBase:     "quay.io/projectquay/quay",
		v1.ComponentClair:    "quay.io/projectquay/clair",
		v1.ComponentRedis:    "centos/redis-32-centos7",
		v1.ComponentPostgres: "centos/postgresql-10-centos7",
	}

	imageOverride := types.Image{
		Name: defaultImagesFor[component],
	}

	image := os.Getenv(envVarFor[component])
	if image == "" {
		return imageOverride, nil
	}

	if len(strings.Split(image, "@")) == 2 {
		imageOverride.NewName = strings.Split(image, "@")[0]
		imageOverride.Digest = strings.Split(image, "@")[1]
	} else if len(strings.Split(image, ":")) == 2 {
		imageOverride.NewName = strings.Split(image, ":")[0]
		imageOverride.NewTag = strings.Split(image, ":")[1]
	} else {
		return types.Image{}, fmt.Errorf(
			"image override must be reference by tag or digest: %s", image,
		)
	}

	return imageOverride, nil
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

func unmanagedTLSOverlayDir() string {
	return filepath.Join(kustomizeDir(), "overlays", "current", "unmanaged-tls")
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

func encode(value interface{}) []byte {
	yamlified, _ := yaml.Marshal(value)
	return yamlified
}

func decode(bytes []byte) interface{} {
	var value interface{}
	_ = yaml.Unmarshal(bytes, &value)
	return value
}

// EnsureCreationOrder sorts the given slice of Kubernetes objects so that when created in order,
// `Deployments` will be created after their dependencies (`Secrets`/`ConfigMaps`/etc).
func EnsureCreationOrder(objects []client.Object) []client.Object {
	sort.Slice(
		objects,
		func(i, j int) bool {
			obji := objects[i]
			return obji.GetObjectKind().GroupVersionKind().Kind != "Deployment"
		},
	)
	return objects
}

// ModelFor returns an empty Kubernetes object instance for the given `GroupVersionKind`.
// Example: Calling with `core.v1.Secret` GVK returns an empty `corev1.Secret` instance.
func ModelFor(gvk schema.GroupVersionKind) client.Object {
	switch gvk.String() {
	case schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}.String():
		return &corev1.Namespace{}
	case schema.GroupVersionKind{Version: "v1", Kind: "Secret"}.String():
		return &corev1.Secret{}
	case schema.GroupVersionKind{Version: "v1", Kind: "Service"}.String():
		return &corev1.Service{}
	case schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}.String():
		return &corev1.ConfigMap{}
	case schema.GroupVersionKind{Version: "v1", Kind: "ServiceAccount"}.String():
		return &corev1.ServiceAccount{}
	case schema.GroupVersionKind{Version: "v1", Kind: "PersistentVolumeClaim"}.String():
		return &corev1.PersistentVolumeClaim{}
	case schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}.String():
		return &apps.Deployment{}
	case schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"}.String():
		return &rbac.Role{}
	case schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"}.String():
		return &rbac.RoleBinding{}
	case schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "Route"}.String():
		return &route.Route{}
	case schema.GroupVersionKind{Group: "objectbucket.io", Version: "v1alpha1", Kind: "ObjectBucketClaim"}.String():
		return &objectbucket.ObjectBucketClaim{}
	case schema.GroupVersionKind{Group: "autoscaling", Version: "v2beta2", Kind: "HorizontalPodAutoscaler"}.String():
		return &autoscaling.HorizontalPodAutoscaler{}
	case schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}.String():
		return &batchv1.Job{}
	case schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}.String():
		return &prometheusv1.ServiceMonitor{}
	case schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PrometheusRule"}.String():
		return &prometheusv1.PrometheusRule{}
	default:
		panic(fmt.Sprintf("Missing model for GVK %s", gvk.String()))
	}
}

// generate uses Kustomize as a library to build the runtime objects to be applied to a cluster.
func generate(
	kustomization *types.Kustomization, overlay string, quayConfigFiles map[string][]byte,
) ([]client.Object, error) {
	fSys := filesys.MakeEmptyDirInMemory()
	if err := filepath.Walk(
		kustomizeDir(),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			f, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			return fSys.WriteFile(path, f)
		},
	); err != nil {
		return nil, err
	}

	// Write `kustomization.yaml` to filesystem
	kustomizationFile, err := yaml.Marshal(kustomization)
	if err != nil {
		return nil, err
	}

	if err := fSys.WriteFile(
		filepath.Join(appDir(), "kustomization.yaml"), kustomizationFile,
	); err != nil {
		return nil, err
	}

	// Add all Quay config files to directory to be included in the generated `Secret`
	for fileName, file := range quayConfigFiles {
		if err := fSys.WriteFile(
			filepath.Join(appDir(), "bundle", fileName), file,
		); err != nil {
			return nil, err
		}
	}

	opts := &krusty.Options{
		PluginConfig: &types.PluginConfig{},
	}
	k := krusty.MakeKustomizer(opts)
	resMap, err := k.Run(fSys, overlay)
	if err != nil {
		return nil, err
	}

	output := []client.Object{}
	for _, resource := range resMap.Resources() {
		resourceJSON, err := resource.MarshalJSON()
		if err != nil {
			return nil, err
		}

		gvk := schema.GroupVersionKind{
			Group:   resource.GetGvk().Group,
			Version: resource.GetGvk().Version,
			Kind:    resource.GetGvk().Kind,
		}

		obj := ModelFor(gvk)
		if obj == nil {
			return nil, fmt.Errorf("kind not supported: %s", gvk.Kind)
		}

		if err := json.Unmarshal(resourceJSON, obj); err != nil {
			return nil, err
		}

		output = append(output, obj)
	}

	return output, nil
}

// KustomizationFor takes a `QuayRegistry` object and generates a Kustomization for it.
func KustomizationFor(
	ctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	quayConfigFiles map[string][]byte,
	overlay string,
) (*types.Kustomization, error) {
	if quay == nil {
		return nil, errors.New("given QuayRegistry should not be nil")
	}

	configFiles := []string{}
	for key := range quayConfigFiles {
		configFiles = append(configFiles, filepath.Join("bundle", key))
	}

	if len(ctx.ConfigEditorPw) == 0 {
		var err error
		if ctx.ConfigEditorPw, err = generateRandomString(16); err != nil {
			return nil, err
		}
	}

	if ctx.DbRootPw == "" {
		rootpw, err := generateRandomString(32)
		if err != nil {
			return nil, err
		}
		ctx.DbRootPw = rootpw
	}

	quayConfigTLSSources := []string{}
	if ctx.ClusterWildcardCert != nil {
		quayConfigTLSSources = append(quayConfigTLSSources, "ocp-cluster-wildcard.cert="+string(ctx.ClusterWildcardCert))
	}
	if ctx.TLSCert != nil {
		quayConfigTLSSources = append(quayConfigTLSSources, "ssl.cert="+string(ctx.TLSCert))
	}
	if ctx.TLSKey != nil {
		quayConfigTLSSources = append(quayConfigTLSSources, "ssl.key="+string(ctx.TLSKey))
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
						"DB_URI=" + ctx.DbUri,
						"DB_ROOT_PW=" + ctx.DbRootPw,
						"CONFIG_EDITOR_PW=" + ctx.ConfigEditorPw,
						"SECURITY_SCANNER_V4_PSK=" + ctx.SecurityScannerV4PSK,
					},
				},
			},
		},
		{
			GeneratorArgs: types.GeneratorArgs{
				Name: v1.QuayConfigTLSSecretName,
				KvPairSources: types.KvPairSources{
					LiteralSources: quayConfigTLSSources,
				},
			},
		},
		{
			GeneratorArgs: types.GeneratorArgs{
				Name: "quay-config-editor-credentials",
				KvPairSources: types.KvPairSources{
					LiteralSources: []string{
						"username=" + configEditorUser,
						"password=" + ctx.ConfigEditorPw,
					},
				},
			},
		},
	}

	componentPaths := []string{}
	if overlay == upgradeOverlayDir() {
		componentPaths = []string{"../components/job"}
	}

	for _, component := range quay.Spec.Components {
		if !component.Managed {
			continue
		}

		componentPaths = append(
			componentPaths, filepath.Join("..", "components", string(component.Kind)),
		)

		componentConfigFiles, err := componentConfigFilesFor(
			ctx, component.Kind, quay, quayConfigFiles,
		)
		if componentConfigFiles == nil || err != nil {
			continue
		}

		sources := []string{}
		for filename, fileValue := range componentConfigFiles {
			sources = append(sources, strings.Join([]string{filename, string(fileValue)}, "="))
		}

		generatedSecrets = append(
			generatedSecrets,
			types.SecretArgs{
				GeneratorArgs: types.GeneratorArgs{
					Name:     string(component.Kind) + "-config-secret",
					Behavior: "merge",
					KvPairSources: types.KvPairSources{
						LiteralSources: sources,
					},
				},
			},
		)
	}

	images := []types.Image{}
	for _, component := range append(
		quay.Spec.Components, v1.Component{Kind: "base", Managed: true},
	) {
		image, err := componentImageFor(component.Kind)
		if err != nil {
			return nil, err
		}
		if image.NewName != "" || image.Digest != "" {
			images = append(images, image)
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

// Inflate takes a `QuayRegistry` object and returns a set of Kubernetes objects representing a
// Quay deployment.
func Inflate(
	ctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	bundle *corev1.Secret,
	log logr.Logger,
	nores bool,
) ([]client.Object, error) {
	// Each managed component brings its own generated `config.yaml` fields which are
	// accumulated under the key <component>.config.yaml and then added to the base `Secret`.
	componentConfigFiles := bundle.DeepCopy().Data

	var parsedUserConfig map[string]interface{}
	if err := yaml.Unmarshal(bundle.Data["config.yaml"], &parsedUserConfig); err != nil {
		return nil, err
	}

	// Generate or pull out the `SECRET_KEY` and `DATABASE_SECRET_KEY`. Since these must be
	// stable across runs of the same config, we store them (and re-read them) from a
	// specialized `Secret`.  TODO(alecmerdler): Refector these three blocks...
	if key, ok := parsedUserConfig["DATABASE_SECRET_KEY"].(string); ok && len(key) > 0 {
		log.Info("`DATABASE_SECRET_KEY` found in user-provided config")
		ctx.DatabaseSecretKey = key
	} else if ctx.DatabaseSecretKey == "" {
		log.Info("`DATABASE_SECRET_KEY` not found in user-provided config, generating")
		databaseSecretKey, err := generateRandomString(secretKeyLength)
		if err != nil {
			return nil, err
		}
		ctx.DatabaseSecretKey = databaseSecretKey
	}
	parsedUserConfig["DATABASE_SECRET_KEY"] = ctx.DatabaseSecretKey

	if key, ok := parsedUserConfig["SECRET_KEY"].(string); ok && len(key) > 0 {
		log.Info("`SECRET_KEY` found in user-provided config")
		ctx.SecretKey = key
	} else if ctx.SecretKey == "" {
		log.Info("`SECRET_KEY` not found in user-provided config, generating")
		secretKey, err := generateRandomString(secretKeyLength)
		if err != nil {
			return nil, err
		}
		ctx.SecretKey = secretKey
	}
	parsedUserConfig["SECRET_KEY"] = ctx.SecretKey

	if dbURI, ok := parsedUserConfig["DB_URI"].(string); ok && len(dbURI) > 0 {
		ctx.DbUri = dbURI
	} else if v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentPostgres) && len(ctx.DbUri) == 0 {
		log.Info("managed `DB_URI` not found in config, generating new one")
		user := quay.GetName() + "-quay-database"
		name := quay.GetName() + "-quay-database"
		host := quay.GetName() + "-quay-database"
		port := "5432"
		password, err := generateRandomString(secretKeyLength)
		if err != nil {
			return nil, err
		}
		ctx.DbUri = fmt.Sprintf(
			"postgresql://%s:%s@%s:%s/%s", user, password, host, port, name,
		)
	}
	parsedUserConfig["DB_URI"] = ctx.DbUri

	for field, value := range BaseConfig() {
		if _, ok := parsedUserConfig[field]; !ok {
			parsedUserConfig[field] = value
		}
	}
	componentConfigFiles["config.yaml"] = encode(parsedUserConfig)

	for _, component := range quay.Spec.Components {
		if !component.Managed {
			continue
		}

		fieldGroup, err := FieldGroupFor(ctx, component.Kind, quay)
		if err != nil {
			return nil, err
		}

		index := fmt.Sprintf("%s.config.yaml", component.Kind)
		componentConfigFiles[index] = encode(fieldGroup)
	}

	log.Info("Ensuring TLS cert/key pair for Quay app")
	tlsCert, tlsKey, err := EnsureTLSFor(ctx, quay)
	if err != nil {
		return nil, err
	}
	ctx.TLSCert = tlsCert
	ctx.TLSKey = tlsKey

	var overlay string
	if rolloutBlocked(quay) {
		overlay = configEditorOnlyOverlay()
	} else if quay.Status.CurrentVersion != v1.QuayVersionCurrent {
		overlay = upgradeOverlayDir()
	} else if !v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentTLS) {
		overlay = unmanagedTLSOverlayDir()
	} else {
		overlay = overlayDir()
	}

	kustomization, err := KustomizationFor(ctx, quay, componentConfigFiles, overlay)
	if err != nil {
		return nil, err
	}

	resources, err := generate(kustomization, overlay, componentConfigFiles)
	if err != nil {
		return nil, err
	}

	for index, resource := range resources {
		obj, err := middleware.Process(quay, resource, nores)
		if err != nil {
			return nil, err
		}

		resources[index] = obj
	}

	for index, resource := range resources {
		resources[index] = v1.EnsureOwnerReference(quay, resource)
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
