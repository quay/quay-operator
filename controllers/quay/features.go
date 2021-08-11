package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	prometheusv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/quay/config-tool/pkg/lib/fieldgroups/hostsettings"
	"gopkg.in/yaml.v2"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/kustomize"
)

const (
	datastoreBucketNameKey = "BUCKET_NAME"
	datastoreBucketHostKey = "BUCKET_HOST"
	datastoreAccessKey     = "AWS_ACCESS_KEY_ID"
	datastoreSecretKey     = "AWS_SECRET_ACCESS_KEY"

	databaseSecretKey         = "DATABASE_SECRET_KEY"
	secretKey                 = "SECRET_KEY"
	dbURI                     = "DB_URI"
	clairSecurityScannerV4PSK = "CLAIR_SECURITY_SCANNER_V4_PSK"
	configEditorPassword      = "CONFIG_EDITOR_PASSWORD"
	postgresRootPassword      = "POSTGRES_ROOT_PASSWORD"

	// GrafanaDashboardConfigNamespace holds the namespace where grafana configs live.
	GrafanaDashboardConfigNamespace = "openshift-config-managed"
)

// checkManagedKeys verifies if a secret containing database access for provided QuayRegistry
// exists.  The secret is expected to contain a label (QuayRegistryNameLabel) set to the name
// of the QuayRegistry object. If found populates database related data in provided
// QuayRegistryContext.
func (r *QuayRegistryReconciler) checkManagedKeys(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	configBundle map[string][]byte,
) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	var secrets corev1.SecretList
	listOptions := &client.ListOptions{
		Namespace: quay.GetNamespace(),
		LabelSelector: labels.SelectorFromSet(
			map[string]string{
				kustomize.QuayRegistryNameLabel: quay.GetName(),
			},
		),
	}

	if err := r.List(ctx, &secrets, listOptions); err != nil {
		return qctx, quay, err
	}

	for _, secret := range secrets.Items {
		if !v1.IsManagedKeysSecretFor(quay, &secret) {
			continue
		}
		qctx.DatabaseSecretKey = string(secret.Data[databaseSecretKey])
		qctx.SecretKey = string(secret.Data[secretKey])
		qctx.DBURI = string(secret.Data[dbURI])
		qctx.ClairSecurityScannerV4PSK = string(secret.Data[clairSecurityScannerV4PSK])
		qctx.ConfigEditorPassword = string(secret.Data[configEditorPassword])
		qctx.PostgresRootPassword = string(secret.Data[postgresRootPassword])
		return qctx, quay, nil
	}

	return qctx, quay, nil
}

// checkManagedTLS verifies if the provided config bundle contains the certificate and key to be
// used by a QuayRegistry. Sets the fields in provided QuayRegistryContext.
func (r *QuayRegistryReconciler) checkManagedTLS(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	configBundle map[string][]byte,
) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	providedTLSCert := configBundle["ssl.cert"]
	providedTLSKey := configBundle["ssl.key"]

	if providedTLSCert == nil || providedTLSKey == nil {
		r.Log.Info("TLS cert/key pair not provided, using default cluster wildcard cert")
		return qctx, quay, nil
	}

	r.Log.Info("provided TLS cert/key pair in `configBundleSecret` will be used")
	qctx.TLSCert = providedTLSCert
	qctx.TLSKey = providedTLSKey
	return qctx, quay, nil
}

// checkRoutesAvailable verifies if the cluster we are running support Routes. It does it by means
// of creating a fake route, if succeeds then attempts to load the certificate used by the cluster
// and populates all info in provided QuayRegistryContext.
func (r *QuayRegistryReconciler) checkRoutesAvailable(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	configBundle map[string][]byte,
) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	// NOTE: The `route` component is unique because we allow users to set `SERVER_HOSTNAME`
	// field instead of controlling the entire fieldgroup. This value is then passed to the
	// created `Route` using a Kustomize variable.
	var config map[string]interface{}
	if err := yaml.Unmarshal(configBundle["config.yaml"], &config); err != nil {
		return qctx, quay, err
	}

	fieldGroup, err := hostsettings.NewHostSettingsFieldGroup(config)
	if err != nil {
		return qctx, quay, err
	}
	if fieldGroup.ServerHostname != "" {
		qctx.ServerHostname = fieldGroup.ServerHostname
	}

	fakeRoute := v1.EnsureOwnerReference(
		quay,
		&routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      quay.GetName() + "-test-route",
				Namespace: quay.GetNamespace(),
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "none",
				},
			},
		},
	)

	if err := r.Client.Create(ctx, fakeRoute); err != nil && !errors.IsAlreadyExists(err) {
		r.Log.Info("cluster does not support `Route` API", "error", err)
		return qctx, quay, nil
	}

	// Wait until `status.ingress` is populated (should be immediately).
	r.Log.Info("cluster supports `Routes` API")
	if err := wait.Poll(
		500*time.Millisecond,
		5*time.Minute,
		func() (done bool, err error) {
			nsn := types.NamespacedName{
				Name:      quay.GetName() + "-test-route",
				Namespace: quay.GetNamespace(),
			}
			if err := r.Client.Get(ctx, nsn, fakeRoute); err != nil {
				return false, client.IgnoreNotFound(err)
			}

			rt := fakeRoute.(*routev1.Route)
			if len(rt.Status.Ingress) == 0 {
				r.Log.Info("waiting to detect `routerCanonicalHostname`")
				return false, nil
			}

			qctx.SupportsRoutes = true
			qctx.ClusterHostname = rt.Status.Ingress[0].RouterCanonicalHostname
			return true, nil
		},
	); err != nil {
		return qctx, quay, err
	}

	r.Log.Info("detected router canonical hostname: " + qctx.ClusterHostname)
	if qctx.ServerHostname == "" {
		qctx.ServerHostname = fmt.Sprintf(
			"%s-quay-%s.%s",
			quay.GetName(),
			quay.GetNamespace(),
			qctx.ClusterHostname,
		)
	}

	// TODO(alecmerdler): Try to fetch the wildcard cert from the `ConfigMap` at
	// `openshift-config-managed/default-ingress-cert`...
	hname := fmt.Sprintf("%s:443", fakeRoute.(*routev1.Route).Spec.Host)
	clusterWildcardCert, err := getCertificatesPEM(hname)
	if err != nil {
		return qctx, quay, err
	}
	qctx.ClusterWildcardCert = clusterWildcardCert
	r.Log.Info("detected cluster wildcard certificate for " + qctx.ClusterHostname)

	if err := r.Client.Delete(ctx, fakeRoute); err != nil {
		return qctx, quay, err
	}
	return qctx, quay, nil
}

// checkObjectBucketClaimsAvailable evaluates the the cluster supports ObjectBucketClaims, it
// attempts to List ObjectBucketClaims, in case of success register this cluster as supporting
// and attempts to load the Storage related data into provided QuayRegistryContext.
func (r *QuayRegistryReconciler) checkObjectBucketClaimsAvailable(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	configBundle map[string][]byte,
) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	datastoreName := types.NamespacedName{
		Namespace: quay.GetNamespace(),
		Name:      quay.GetName() + "-quay-datastore",
	}

	var objectBucketClaims objectbucket.ObjectBucketClaimList
	if err := r.Client.List(ctx, &objectBucketClaims); err != nil {
		r.Log.Info("cluster does not support `ObjectBucketClaim` API")
		return qctx, quay, nil
	}
	r.Log.Info("cluster supports `ObjectBucketClaims` API")
	qctx.SupportsObjectStorage = true

	for _, obc := range objectBucketClaims.Items {
		if obc.GetNamespace()+"/"+obc.GetName() != datastoreName.String() {
			continue
		}
		r.Log.Info("`ObjectBucketClaim` exists")

		var datastoreSecret corev1.Secret
		if err := r.Client.Get(ctx, datastoreName, &datastoreSecret); err != nil {
			r.Log.Error(err, "unable to retrieve Quay datastore `Secret`")
			return qctx, quay, err
		}

		var datastoreConfig corev1.ConfigMap
		if err := r.Client.Get(ctx, datastoreName, &datastoreConfig); err != nil {
			r.Log.Error(err, "unable to retrieve Quay datastore `ConfigMap`")
			return qctx, quay, err
		}
		r.Log.Info("found `ObjectBucketClaim` and credentials `Secret`, `ConfigMap`")

		host := string(datastoreConfig.Data[datastoreBucketHostKey])
		if strings.Contains(host, ".svc") && !strings.Contains(host, ".svc.cluster.local") {
			r.Log.Info("`ObjectBucketClaim` using in-cluster endpoint, ensuring FQDN")
			host = strings.ReplaceAll(host, ".svc", ".svc.cluster.local")
		}

		qctx.StorageBucketName = string(datastoreConfig.Data[datastoreBucketNameKey])
		qctx.StorageHostname = host
		qctx.StorageAccessKey = string(datastoreSecret.Data[datastoreAccessKey])
		qctx.StorageSecretKey = string(datastoreSecret.Data[datastoreSecretKey])
		qctx.ObjectStorageInitialized = true
		return qctx, quay, nil
	}

	r.Log.Info("`ObjectBucketClaim` not found")
	return qctx, quay, nil
}

// checkBuildManagerAvailable verifies if the provided configBundle contains an entry pointing to
// the buildman hostname. Sets the property in provided QuayRegistryContext.
// TODO: Improve this once `builds` is a managed component.
func (r *QuayRegistryReconciler) checkBuildManagerAvailable(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	configBundle map[string][]byte,
) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal(configBundle["config.yaml"], &config); err != nil {
		return qctx, quay, err
	}

	if buildManagerHostname, ok := config["BUILDMAN_HOSTNAME"]; ok {
		qctx.BuildManagerHostname = buildManagerHostname.(string)
	}

	return qctx, quay, nil
}

// checkMonitoringAvailable validates if the monitoring component can be run. We assume that we
// are running in an Openshift environment with cluster monitoring enabled for our monitoring
// component to work
func (r *QuayRegistryReconciler) checkMonitoringAvailable(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	configBundle map[string][]byte,
) (*quaycontext.QuayRegistryContext, *v1.QuayRegistry, error) {
	if len(r.WatchNamespace) > 0 {
		msg := "monitoring is only supported in AllNamespaces mode. Disabling it"
		r.Log.Info(msg)
		return qctx, quay, fmt.Errorf(msg)
	}

	var serviceMonitors prometheusv1.ServiceMonitorList
	if err := r.Client.List(ctx, &serviceMonitors); err != nil {
		r.Log.Info("Unable to find ServiceMonitor CRD. Monitoring component disabled")
		return qctx, quay, err
	}
	r.Log.Info("cluster supports `ServiceMonitor` API")

	var prometheusRules prometheusv1.PrometheusRuleList
	if err := r.Client.List(ctx, &prometheusRules); err != nil {
		r.Log.Info("Unable to find PrometheusRule CRD. Monitoring component disabled")
		return qctx, quay, err
	}
	r.Log.Info("cluster supports `PrometheusRules` API")

	nsn := types.NamespacedName{Name: GrafanaDashboardConfigNamespace}
	var grafanaDashboardNamespace corev1.Namespace
	if err := r.Client.Get(ctx, nsn, &grafanaDashboardNamespace); err != nil {
		msg := fmt.Sprintf(
			"Unable to find the Grafana config namespace %s. Monitoring disabled",
			GrafanaDashboardConfigNamespace,
		)
		r.Log.Info(msg)
		return qctx, quay, err
	}
	r.Log.Info(GrafanaDashboardConfigNamespace + " found")
	qctx.SupportsMonitoring = true

	return qctx, quay, nil
}

// configEditorCredentialsSecretFrom attempts to find the secret that holds the config editor
// credentials among the provided objects. Returns the name of the secret or empty string if the
// object could not be found.
func configEditorCredentialsSecretFrom(objs []client.Object) string {
	for _, obj := range objs {
		groupVersionKind := obj.GetObjectKind().GroupVersionKind().String()
		secretGVK := schema.GroupVersionKind{Version: "v1", Kind: "Secret"}.String()

		if groupVersionKind != secretGVK {
			continue
		}

		if !strings.Contains(obj.GetName(), "quay-config-editor-credentials") {
			continue
		}

		return obj.GetName()
	}
	return ""
}

// getCertificatesPEM returns the certificate used by the server at 'address'.
// Taken from https://stackoverflow.com/questions/46735347/how-can-i-fetch-a-certificate-from-a-url
func getCertificatesPEM(address string) ([]byte, error) {
	conn, err := tls.Dial("tcp", address, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var b bytes.Buffer
	for _, cert := range conn.ConnectionState().PeerCertificates {
		if err := pem.Encode(
			&b, &pem.Block{
				Type:  "CERTIFICATE",
				Bytes: cert.Raw,
			},
		); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}
