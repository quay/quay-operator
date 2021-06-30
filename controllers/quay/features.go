package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/quay/config-tool/pkg/lib/fieldgroups/hostsettings"
	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/kustomize"
)

const (
	datastoreBucketNameKey = "BUCKET_NAME"
	datastoreBucketHostKey = "BUCKET_HOST"
	datastoreAccessKey     = "AWS_ACCESS_KEY_ID"
	datastoreSecretKey     = "AWS_SECRET_ACCESS_KEY"

	databaseSecretKey = "DATABASE_SECRET_KEY"
	secretKey         = "SECRET_KEY"

	GrafanaDashboardConfigNamespace = "openshift-config-managed"
)

func (r *QuayRegistryReconciler) checkManagedKeys(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, rawConfig []byte) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	var secrets corev1.SecretList
	listOptions := &client.ListOptions{
		Namespace: quay.GetNamespace(),
		LabelSelector: labels.SelectorFromSet(map[string]string{
			kustomize.QuayRegistryNameLabel: quay.GetName(),
		}),
	}

	if err := r.List(context.Background(), &secrets, listOptions); err != nil {
		return ctx, quay, err
	}

	for _, secret := range secrets.Items {
		if v1.IsManagedKeysSecretFor(quay, &secret) {
			ctx.DatabaseSecretKey = string(secret.Data[databaseSecretKey])
			ctx.SecretKey = string(secret.Data[secretKey])
			break
		}
	}

	return ctx, quay, nil
}

func (r *QuayRegistryReconciler) checkRoutesAvailable(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, rawConfig []byte) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	fakeRoute, err := v1.EnsureOwnerReference(quay, &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      quay.GetName() + "-test-route",
			Namespace: quay.GetNamespace(),
		},
		Spec: routev1.RouteSpec{To: routev1.RouteTargetReference{Kind: "Service", Name: "none"}},
	})

	if err != nil {
		return ctx, quay, err
	}

	if err := r.Client.Create(context.Background(), fakeRoute); err == nil || errors.IsAlreadyExists(err) {
		r.Log.Info("cluster supports `Routes` API")

		// Wait until `status.ingress` is populated (should be immediately).
		err = wait.Poll(500*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
			if err := r.Client.Get(context.Background(), types.NamespacedName{Name: quay.GetName() + "-test-route", Namespace: quay.GetNamespace()}, fakeRoute); err != nil {
				return false, client.IgnoreNotFound(err)
			}

			if len(fakeRoute.(*routev1.Route).Status.Ingress) > 0 {
				ctx.SupportsRoutes = true
				ctx.ClusterHostname = fakeRoute.(*routev1.Route).Status.Ingress[0].RouterCanonicalHostname

				return true, nil
			}

			r.Log.Info("waiting to detect `routerCanonicalHostname`")

			return false, nil
		})
		if err != nil {
			return ctx, quay, err
		}

		// NOTE: The `route` component is unique because we allow users to set the `SERVER_HOSTNAME` field instead of controlling the entire fieldgroup.
		// This value is then passed to the created `Route` using a Kustomize variable.
		var config map[string]interface{}
		if err := yaml.Unmarshal(rawConfig, &config); err != nil {
			return ctx, quay, err
		}

		fieldGroup, err := hostsettings.NewHostSettingsFieldGroup(config)
		if err != nil {
			return ctx, quay, err
		}

		if fieldGroup.ServerHostname != "" {
			ctx.ServerHostname = fieldGroup.ServerHostname
		} else {
			ctx.ServerHostname = strings.Join([]string{
				strings.Join([]string{quay.GetName(), "quay", quay.GetNamespace()}, "-"),
				ctx.ClusterHostname},
				".")
		}

		r.Log.Info("detected router canonical hostname: " + ctx.ClusterHostname)

		if err := r.Client.Delete(context.Background(), fakeRoute); err != nil {
			return ctx, quay, err
		}

		return ctx, quay, nil
	}

	r.Log.Info("cluster does not support `Route` API", "error", err)

	return ctx, quay, nil
}

func (r *QuayRegistryReconciler) checkObjectBucketClaimsAvailable(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, rawConfig []byte) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	datastoreName := types.NamespacedName{Namespace: quay.GetNamespace(), Name: quay.GetName() + "-quay-datastore"}
	var objectBucketClaims objectbucket.ObjectBucketClaimList
	if err := r.Client.List(context.Background(), &objectBucketClaims); err == nil {
		r.Log.Info("cluster supports `ObjectBucketClaims` API")

		ctx.SupportsObjectStorage = true

		found := false
		for _, obc := range objectBucketClaims.Items {
			if obc.GetNamespace()+"/"+obc.GetName() == datastoreName.String() {
				found = true
				r.Log.Info("`ObjectBucketClaim` exists")

				var datastoreSecret corev1.Secret
				if err = r.Client.Get(context.Background(), datastoreName, &datastoreSecret); err != nil {
					r.Log.Error(err, "unable to retrieve Quay datastore `Secret`")

					return ctx, quay, err
				}

				var datastoreConfig corev1.ConfigMap
				if err = r.Client.Get(context.Background(), datastoreName, &datastoreConfig); err != nil {
					r.Log.Error(err, "unable to retrieve Quay datastore `ConfigMap`")

					return ctx, quay, err
				}

				r.Log.Info("found `ObjectBucketClaim` and credentials `Secret`, `ConfigMap`")

				host := string(datastoreConfig.Data[datastoreBucketHostKey])
				if strings.Contains(host, ".svc") && !strings.Contains(host, ".svc.cluster.local") {
					r.Log.Info("`ObjectBucketClaim` is using in-cluster endpoint, ensuring we use the fully qualified domain name")
					host = strings.ReplaceAll(host, ".svc", ".svc.cluster.local")
				}

				ctx.StorageBucketName = string(datastoreConfig.Data[datastoreBucketNameKey])
				ctx.StorageHostname = host
				ctx.StorageAccessKey = string(datastoreSecret.Data[datastoreAccessKey])
				ctx.StorageSecretKey = string(datastoreSecret.Data[datastoreSecretKey])
				ctx.ObjectStorageInitialized = true
			}
		}

		if !found {
			r.Log.Info("`ObjectBucketClaim` not found")
		}

	} else if err != nil {
		r.Log.Info("cluster does not support `ObjectBucketClaim` API")
	}

	return ctx, quay, nil
}

// TODO: Improve this once `builds` is a managed component.
func (r *QuayRegistryReconciler) checkBuildManagerAvailable(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, rawConfig []byte) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal(rawConfig, &config); err != nil {
		return ctx, quay, err
	}

	if buildManagerHostname, ok := config["BUILDMAN_HOSTNAME"]; ok {
		ctx.BuildManagerHostname = buildManagerHostname.(string)
	}

	return ctx, quay, nil
}

// Validates if the monitoring component can be run. We assume that we are
// running in an Openshift environment with cluster monitoring enabled for our
// monitoring component to work
func (r *QuayRegistryReconciler) checkMonitoringAvailable(ctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, rawConfig []byte) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	if len(r.WatchNamespace) > 0 {
		msg := "monitoring is only supported in AllNamespaces mode. Disabling component monitoring"
		r.Log.Info(msg)
		err := fmt.Errorf(msg)
		return ctx, quay, err
	}

	var serviceMonitors prometheusv1.ServiceMonitorList
	if err := r.Client.List(context.Background(), &serviceMonitors); err != nil {
		r.Log.Info("Unable to find ServiceMonitor CRD. Monitoring component disabled")
		return ctx, quay, err
	}
	r.Log.Info("cluster supports `ServiceMonitor` API")

	var prometheusRules prometheusv1.PrometheusRuleList
	if err := r.Client.List(context.Background(), &prometheusRules); err != nil {
		r.Log.Info("Unable to find PrometheusRule CRD. Monitoring component disabled")
		return ctx, quay, err
	}
	r.Log.Info("cluster supports `PrometheusRules` API")

	namespaceKey := types.NamespacedName{Name: GrafanaDashboardConfigNamespace}
	var grafanaDashboardNamespace corev1.Namespace
	if err := r.Client.Get(context.Background(), namespaceKey, &grafanaDashboardNamespace); err != nil {
		msg := fmt.Sprintf("Unable to find the Grafana config namespace %s. Monitoring component disabled", GrafanaDashboardConfigNamespace)
		r.Log.Info(msg)
		return ctx, quay, err
	}
	r.Log.Info(GrafanaDashboardConfigNamespace + " found")

	ctx.SupportsMonitoring = true

	return ctx, quay, nil
}

func configEditorCredentialsSecretFrom(objs []client.Object) string {
	for _, obj := range objs {
		objectMeta, _ := meta.Accessor(obj)
		groupVersionKind := obj.GetObjectKind().GroupVersionKind().String()
		secretGVK := schema.GroupVersionKind{Version: "v1", Kind: "Secret"}.String()

		if groupVersionKind == secretGVK && strings.Contains(objectMeta.GetName(), "quay-config-editor-credentials") {
			return objectMeta.GetName()
		}
	}

	return ""
}
