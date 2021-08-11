package kustomize

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-logr/logr"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	route "github.com/openshift/api/route/v1"
	apps "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1beta1"
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
	"github.com/quay/quay-operator/pkg/middleware"
)

const (
	// QuayRegistryNameLabel is the label we add to all objects created through Kustomize.
	QuayRegistryNameLabel = "quay-operator/quayregistry"

	configSecretPrefix                = "quay-config-secret"
	registryHostnameAnnotation        = "quay-registry-hostname"
	buildManagerHostnameAnnotation    = "quay-buildmanager-hostname"
	managedFieldGroupsAnnotation      = "quay-managed-fieldgroups"
	operatorServiceEndpointAnnotation = "quay-operator-service-endpoint"
	podNamespaceKey                   = "MY_POD_NAMESPACE"
	componentImagePrefix              = "RELATED_IMAGE_COMPONENT_"
	configEditorUser                  = "quayconfig"
)

// componentImageFor checks for an environment variable indicating which component container image
// to use. If set, returns a Kustomize image override for the given component.
func componentImageFor(component v1.ComponentKind) types.Image {
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
		return imageOverride
	}

	if len(strings.Split(image, "@")) == 2 {
		imageOverride.NewName = strings.Split(image, "@")[0]
		imageOverride.Digest = strings.Split(image, "@")[1]
	} else if len(strings.Split(image, ":")) == 2 {
		imageOverride.NewName = strings.Split(image, ":")[0]
		imageOverride.NewTag = strings.Split(image, ":")[1]
	} else {
		panic("image override must be reference by tag or manifest digest: " + image)
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
	cond := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeRolloutBlocked)
	return cond != nil && cond.Status == metav1.ConditionTrue
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

// EnsureCreationOrder sorts the given slice of Kubernetes objects so that when created in order,
// `Deployments` will be created after their dependencies (`Secrets`/`ConfigMaps`/etc).
func EnsureCreationOrder(objects []client.Object) []client.Object {
	sort.Slice(
		objects,
		func(i, j int) bool {
			objType := objects[i].GetObjectKind().GroupVersionKind()
			deploy := schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			}
			return objType.String() != deploy.String()
		},
	)
	return objects
}

// ModelFor returns an empty Kubernetes object instance for the given `GroupVersionKind`.
// Example: Calling with `core.v1.Secret` GVK returns an empty `corev1.Secret` instance.
// This function returns nil if the GVK is not mapped to a concrete implementation.
func ModelFor(gvk resid.Gvk) client.Object {
	switch gvk {
	case resid.Gvk{
		Version: "v1",
		Kind:    "Namespace",
	}:
		return &corev1.Namespace{}
	case resid.Gvk{
		Version: "v1",
		Kind:    "Secret",
	}:
		return &corev1.Secret{}
	case resid.Gvk{
		Version: "v1",
		Kind:    "Service",
	}:
		return &corev1.Service{}
	case resid.Gvk{
		Version: "v1",
		Kind:    "ConfigMap",
	}:
		return &corev1.ConfigMap{}
	case resid.Gvk{
		Version: "v1",
		Kind:    "ServiceAccount",
	}:
		return &corev1.ServiceAccount{}
	case resid.Gvk{
		Version: "v1",
		Kind:    "PersistentVolumeClaim",
	}:
		return &corev1.PersistentVolumeClaim{}
	case resid.Gvk{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}:
		return &apps.Deployment{}
	case resid.Gvk{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1beta1",
		Kind:    "Role",
	}:
		return &rbac.Role{}
	case resid.Gvk{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1beta1",
		Kind:    "RoleBinding",
	}:
		return &rbac.RoleBinding{}
	case resid.Gvk{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	}:
		return &route.Route{}
	case resid.Gvk{
		Group:   "objectbucket.io",
		Version: "v1alpha1",
		Kind:    "ObjectBucketClaim",
	}:
		return &objectbucket.ObjectBucketClaim{}
	case resid.Gvk{
		Group:   "autoscaling",
		Version: "v2beta2",
		Kind:    "HorizontalPodAutoscaler",
	}:
		return &autoscaling.HorizontalPodAutoscaler{}
	case resid.Gvk{
		Group:   "batch",
		Version: "v1",
		Kind:    "Job",
	}:
		return &batchv1.Job{}
	case resid.Gvk{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "ServiceMonitor",
	}:
		return &prometheusv1.ServiceMonitor{}
	case resid.Gvk{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "PrometheusRule",
	}:
		return &prometheusv1.PrometheusRule{}
	default:
		return nil
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
		filepath.Join(appDir(), "kustomization.yaml"),
		kustomizationFile,
	); err != nil {
		return nil, err
	}

	// Add all Quay config files to directory to be included in the generated `Secret`
	for fileName, file := range quayConfigFiles {
		if err = fSys.WriteFile(
			filepath.Join(appDir(), "bundle", fileName),
			file,
		); err != nil {
			return nil, err
		}
	}

	opts := &krusty.Options{}
	k := krusty.MakeKustomizer(fSys, opts)
	resMap, err := k.Run(overlay)
	if err != nil {
		return nil, err
	}

	output := []client.Object{}
	for _, resource := range resMap.Resources() {
		resourceJSON, err := resource.MarshalJSON()
		if err != nil {
			return nil, err
		}

		obj := ModelFor(resource.GetGvk())
		if obj == nil {
			return nil, fmt.Errorf("unmapped GVK: %s", resource.GetGvk().String())
		}

		if err = json.Unmarshal(resourceJSON, obj); err != nil {
			return nil, err
		}

		output = append(output, obj)
	}
	return output, nil
}

// KustomizationFor takes a `QuayRegistry` object and generates a Kustomization for it.
func KustomizationFor(
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	quayConfigFiles map[string][]byte,
) (*types.Kustomization, error) {
	if quay == nil {
		return nil, fmt.Errorf("given QuayRegistry should not be nil")
	}

	configFiles := []string{}
	for key := range quayConfigFiles {
		configFiles = append(configFiles, filepath.Join("bundle", key))
	}

	if qctx.ConfigEditorPassword == "" {
		configEditorPassword, err := generateRandomString(16)
		if err != nil {
			return nil, err
		}
		qctx.ConfigEditorPassword = configEditorPassword
	}

	if qctx.PostgresRootPassword == "" {
		rootpw, err := generateRandomString(32)
		if err != nil {
			return nil, err
		}
		qctx.PostgresRootPassword = rootpw
	}

	quayConfigTLSSources := []string{}
	if qctx.ClusterWildcardCert != nil {
		quayConfigTLSSources = append(
			quayConfigTLSSources,
			fmt.Sprintf("ocp-cluster-wildcard.cert=%s", qctx.ClusterWildcardCert),
		)
	}

	if qctx.TLSCert != nil {
		quayConfigTLSSources = append(
			quayConfigTLSSources,
			fmt.Sprintf("ssl.cert=%s", qctx.TLSCert),
		)
	}

	if qctx.TLSKey != nil {
		quayConfigTLSSources = append(
			quayConfigTLSSources,
			fmt.Sprintf("ssl.key=%s", qctx.TLSKey),
		)
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
						fmt.Sprintf(
							"DATABASE_SECRET_KEY=%s",
							qctx.DatabaseSecretKey,
						),
						fmt.Sprintf(
							"SECRET_KEY=%s",
							qctx.SecretKey,
						),
						fmt.Sprintf(
							"DB_URI=%s",
							qctx.DBURI,
						),
						fmt.Sprintf(
							"CLAIR_SECURITY_SCANNER_V4_PSK=%s",
							qctx.ClairSecurityScannerV4PSK,
						),
						fmt.Sprintf(
							"CONFIG_EDITOR_PASSWORD=%s",
							qctx.ConfigEditorPassword,
						),
						fmt.Sprintf(
							"POSTGRES_ROOT_PASSWORD=%s",
							qctx.PostgresRootPassword,
						),
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
						fmt.Sprintf(
							"username=%s",
							configEditorUser,
						),
						fmt.Sprintf(
							"password=%s",
							qctx.ConfigEditorPassword,
						),
					},
				},
			},
		},
	}

	componentPaths := []string{}
	managedFieldGroups := []string{}
	for _, component := range quay.Spec.Components {
		if !component.Managed {
			continue
		}

		componentPaths = append(
			componentPaths,
			filepath.Join("..", "components", string(component.Kind)),
		)
		managedFieldGroups = append(
			managedFieldGroups,
			fieldGroupNameFor(component.Kind),
		)

		componentConfigFiles, err := componentConfigFilesFor(
			qctx, component.Kind, quay, quayConfigFiles,
		)
		if componentConfigFiles == nil || err != nil {
			continue
		}

		sources := []string{}
		for filename, fileValue := range componentConfigFiles {
			sources = append(
				sources,
				strings.Join([]string{filename, string(fileValue)}, "="),
			)
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
	for _, component := range append(quay.Spec.Components, v1.Component{Kind: "base", Managed: true}) {
		if component.Managed {
			if image := componentImageFor(component.Kind); image.NewName != "" || image.Digest != "" {
				images = append(images, componentImageFor(component.Kind))
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
			registryHostnameAnnotation:        qctx.ServerHostname,
			buildManagerHostnameAnnotation:    strings.Split(qctx.BuildManagerHostname, ":")[0],
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
	configBundle *corev1.Secret,
	log logr.Logger,
) ([]client.Object, error) {
	var err error

	// Each managed component brings its own generated `config.yaml` fields which are
	// accumulated under the key `<component>.config.yaml` and then added to the base
	// `Secret`.
	componentConfigFiles := configBundle.DeepCopy().Data

	var config map[string]interface{}
	if err = yaml.Unmarshal(configBundle.Data["config.yaml"], &config); err != nil {
		return nil, err
	}

	// Generate or pull out the `SECRET_KEY` and `DATABASE_SECRET_KEY`. Since these must be
	// stable across runs of the same config, we store them (and re-read them) from a
	// specialized `Secret`. TODO(alecmerdler): Refector these three blocks...
	if ctx.DatabaseSecretKey == "" {
		ctx.DatabaseSecretKey, err = generateRandomString(secretKeyLength)
		if err != nil {
			return nil, err
		}
		if key, found := config["DATABASE_SECRET_KEY"]; found {
			log.Info("`DATABASE_SECRET_KEY` found in user-provided config")
			ctx.DatabaseSecretKey = key.(string)
		}
	}
	config["DATABASE_SECRET_KEY"] = ctx.DatabaseSecretKey

	if ctx.SecretKey == "" {
		ctx.SecretKey, err = generateRandomString(secretKeyLength)
		if err != nil {
			return nil, err
		}
		if key, found := config["SECRET_KEY"]; found {
			log.Info("`SECRET_KEY` found in user-provided config")
			ctx.SecretKey = key.(string)
		}
	}
	config["SECRET_KEY"] = ctx.SecretKey

	if ctx.DBURI == "" {
		if v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentPostgres) {
			log.Info("managed `DB_URI` not found in config, generating new one")
			user := quay.GetName() + "-quay-database"
			name := quay.GetName() + "-quay-database"
			host := quay.GetName() + "-quay-database"
			port := "5432"
			password, err := generateRandomString(secretKeyLength)
			if err != nil {
				return nil, err
			}

			ctx.DBURI = fmt.Sprintf(
				"postgresql://%s:%s@%s:%s/%s", user, password, host, port, name,
			)
		} else {
			if dbURI, found := config["DB_URI"]; found {
				ctx.DBURI = dbURI.(string)
			}
		}
	}
	config["DB_URI"] = ctx.DBURI

	for field, value := range BaseConfig() {
		if _, ok := config[field]; !ok {
			config[field] = value
		}
	}
	componentConfigFiles["config.yaml"] = encode(config)

	for _, component := range quay.Spec.Components {
		if !component.Managed {
			continue
		}

		fieldGroup, err := FieldGroupFor(ctx, component.Kind, quay)
		if err != nil {
			return nil, err
		}
		componentConfigFiles[string(component.Kind)+".config.yaml"] = encode(fieldGroup)
	}

	log.Info("Ensuring TLS cert/key pair for Quay app")
	if err := EnsureTLSFor(ctx, quay); err != nil {
		return nil, err
	}

	kustomization, err := KustomizationFor(ctx, quay, componentConfigFiles)
	if err != nil {
		return nil, err
	}

	var overlay string
	if rolloutBlocked(quay) {
		overlay = configEditorOnlyOverlay()
	} else if quay.Status.CurrentVersion != v1.QuayVersionCurrent {
		overlay = upgradeOverlayDir()
	} else {
		overlay = overlayDir()
	}
	resources, err := generate(kustomization, overlay, componentConfigFiles)
	if err != nil {
		return nil, err
	}

	for index, resource := range resources {
		obj, err := middleware.Process(ctx, quay, resource)
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
