package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	objectbucket "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	databaseSecretKey    = "DATABASE_SECRET_KEY"
	secretKey            = "SECRET_KEY"
	dbURI                = "DB_URI"
	dbRootPw             = "DB_ROOT_PW"
	configEditorPw       = "CONFIG_EDITOR_PW"
	securityScannerV4PSK = "SECURITY_SCANNER_V4_PSK"
)

// checkDeprecatedManagedKeys populates the provided quay context with information we
// persist between Reconcile calls. This function uses the old secret (<=3.6.4) and not
// the new one (>=3.7.0).
func (r *QuayRegistryReconciler) checkDeprecatedManagedKeys(
	ctx context.Context, qctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry,
) error {
	listOptions := &client.ListOptions{
		Namespace: quay.GetNamespace(),
		LabelSelector: labels.SelectorFromSet(
			map[string]string{
				kustomize.QuayRegistryNameLabel: quay.GetName(),
			},
		),
	}

	var secrets corev1.SecretList
	if err := r.List(ctx, &secrets, listOptions); err != nil {
		return err
	}

	for _, secret := range secrets.Items {
		if !v1.IsManagedKeysSecretFor(quay, &secret) {
			continue
		}

		qctx.DatabaseSecretKey = string(secret.Data[databaseSecretKey])
		qctx.SecretKey = string(secret.Data[secretKey])
		qctx.DbUri = string(secret.Data[dbURI])
		qctx.DbRootPw = string(secret.Data[dbRootPw])
		qctx.ConfigEditorPw = string(secret.Data[configEditorPw])
		qctx.SecurityScannerV4PSK = string(secret.Data[securityScannerV4PSK])
		break
	}

	return nil
}

// checkManagedKeys populates the provided QuayRegistryContext with the information we
// persist in between Reconcile calls. The information kept from one Reconcile loop to
// the next is stored in a secret.
func (r *QuayRegistryReconciler) checkManagedKeys(
	ctx context.Context, qctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry,
) error {

	nsn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", quay.Name, v1.ManagedKeysName),
		Namespace: quay.Namespace,
	}

	var secret corev1.Secret
	if err := r.Get(ctx, nsn, &secret); err != nil {
		if errors.IsNotFound(err) {
			return r.checkDeprecatedManagedKeys(ctx, qctx, quay)
		}
		return err
	}

	qctx.DatabaseSecretKey = string(secret.Data[databaseSecretKey])
	qctx.SecretKey = string(secret.Data[secretKey])
	qctx.DbUri = string(secret.Data[dbURI])
	qctx.DbRootPw = string(secret.Data[dbRootPw])
	qctx.ConfigEditorPw = string(secret.Data[configEditorPw])
	qctx.SecurityScannerV4PSK = string(secret.Data[securityScannerV4PSK])
	return nil
}

// checkManagedTLS verifies if provided bundle contains entries for ssl key and cert,
// populate the data in provided QuayRegistryContext if found.
func (r *QuayRegistryReconciler) checkManagedTLS(
	qctx *quaycontext.QuayRegistryContext, bundle *corev1.Secret,
) {
	providedTLSCert := bundle.Data["ssl.cert"]
	providedTLSKey := bundle.Data["ssl.key"]

	if len(providedTLSCert) == 0 || len(providedTLSKey) == 0 {
		r.Log.Info("TLS cert/key pair not provided, using default cluster wildcard cert")
		return
	}

	r.Log.Info("provided TLS cert/key pair in `configBundleSecret` will be used")
	qctx.TLSCert = providedTLSCert
	qctx.TLSKey = providedTLSKey
}

// checkRoutesAvailable checks if the cluster supports Route objects. // XXX here
// be dragons. This functions attempts to create a fake route and then read the
// certificate used on it, this should be refactored. This is as wrong as it can
// get.
func (r *QuayRegistryReconciler) checkRoutesAvailable(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
	bundle *corev1.Secret,
) error {
	// NOTE: The `route` component is unique because we allow users to set the
	// `SERVER_HOSTNAME` field instead of controlling the entire fieldgroup. This
	// value is then passed to the created `Route` using a Kustomize variable.
	var config map[string]interface{}
	if err := yaml.Unmarshal(bundle.Data["config.yaml"], &config); err != nil {
		return fmt.Errorf("unable to parse config.yaml: %w", err)
	}

	fieldGroup, err := hostsettings.NewHostSettingsFieldGroup(config)
	if err != nil {
		return err
	}

	if fieldGroup.ServerHostname != "" {
		qctx.ServerHostname = fieldGroup.ServerHostname
	}

	fakeRoute := v1.EnsureOwnerReference(
		quay,
		&routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-test-route", quay.GetName()),
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

	// if we fail to create the fake route and the failure is not IsAlreadyExists then
	// we consider as if the cluster does not support Route objects. XXX This must be
	// redesigned. I am keeping the logic as is but we shouldn't blindly trust that any
	// error means "does not support routes".
	if err := r.Client.Create(ctx, fakeRoute); err != nil && !errors.IsAlreadyExists(err) {
		r.Log.Info("cluster does not support `Route` API", "error", err)
		return nil
	}

	// Wait until `status.ingress` is populated (should be immediately).
	r.Log.Info("cluster supports `Routes` API")
	var rt routev1.Route
	if err := wait.Poll(
		time.Second,
		5*time.Minute,
		func() (done bool, err error) {
			routensn := types.NamespacedName{
				Name:      fmt.Sprintf("%s-test-route", quay.GetName()),
				Namespace: quay.GetNamespace(),
			}

			if err := r.Client.Get(ctx, routensn, &rt); err != nil {
				return true, err
			}

			if len(rt.Status.Ingress) == 0 {
				r.Log.Info("waiting to detect `routerCanonicalHostname`")
				return false, nil
			}

			qctx.SupportsRoutes = true
			prefix := fmt.Sprintf("router-%s.", rt.Status.Ingress[0].RouterName)
			cnname := rt.Status.Ingress[0].RouterCanonicalHostname
			qctx.ClusterHostname = strings.TrimPrefix(cnname, prefix)
			r.Log.Info("Detected cluster hostname " + qctx.ClusterHostname)
			return true, nil
		},
	); err != nil {
		return err
	}

	if qctx.ServerHostname == "" {
		qctx.ServerHostname = fmt.Sprintf(
			"%s-quay-%s.%s",
			quay.GetName(),
			quay.GetNamespace(),
			qctx.ClusterHostname,
		)
	}

	wildcard, err := getCertificatesPEM(rt.Spec.Host + ":443")
	if err != nil {
		return err
	}
	qctx.ClusterWildcardCert = wildcard

	return r.Client.Delete(ctx, &rt)
}

func (r *QuayRegistryReconciler) checkObjectBucketClaimsAvailable(
	ctx context.Context, qctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry,
) error {
	dstorensn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-quay-datastore", quay.GetName()),
		Namespace: quay.GetNamespace(),
	}

	var claims objectbucket.ObjectBucketClaimList
	if err := r.Client.List(ctx, &claims); err != nil {
		return fmt.Errorf("unable to list object bucket claims: %s", err)
	}

	qctx.SupportsObjectStorage = true
	r.Log.Info("cluster supports `ObjectBucketClaims` API")

	for _, obc := range claims.Items {
		if obc.GetNamespace()+"/"+obc.GetName() != dstorensn.String() {
			continue
		}

		r.Log.Info("`ObjectBucketClaim` exists")

		var datastoreSecret corev1.Secret
		if err := r.Client.Get(ctx, dstorensn, &datastoreSecret); err != nil {
			r.Log.Error(err, "error retrieving secret, bucket claim not ready")
			return fmt.Errorf("awaiting for bucket claim to be processed")
		}

		var datastoreConfig corev1.ConfigMap
		if err := r.Client.Get(ctx, dstorensn, &datastoreConfig); err != nil {
			r.Log.Error(err, "error retrieving config map, bucket claim not ready")
			return fmt.Errorf("awaiting for bucket claim to be processed")
		}

		r.Log.Info("found `ObjectBucketClaim` and credentials `Secret`, `ConfigMap`")
		host := string(datastoreConfig.Data[datastoreBucketHostKey])
		if strings.Contains(host, ".svc") {
			if !strings.Contains(host, ".svc.cluster.local") {
				host = strings.ReplaceAll(host, ".svc", ".svc.cluster.local")
			}
		}

		qctx.StorageBucketName = string(datastoreConfig.Data[datastoreBucketNameKey])
		qctx.StorageHostname = host
		qctx.StorageAccessKey = string(datastoreSecret.Data[datastoreAccessKey])
		qctx.StorageSecretKey = string(datastoreSecret.Data[datastoreSecretKey])
		qctx.ObjectStorageInitialized = true
		return nil
	}

	r.Log.Info("`ObjectBucketClaim` not found")
	return nil
}

// checkBuildManagerAvailable verifies if the config bundle contains an entry pointing to the
// buildman host. If it contains then sets it properly in the provided QuayRegistryContext
func (r *QuayRegistryReconciler) checkBuildManagerAvailable(
	qctx *quaycontext.QuayRegistryContext, bundle *corev1.Secret,
) error {
	var config map[string]interface{}
	if err := yaml.Unmarshal(bundle.Data["config.yaml"], &config); err != nil {
		return err
	}

	if buildManagerHostname, ok := config["BUILDMAN_HOSTNAME"]; ok {
		qctx.BuildManagerHostname = buildManagerHostname.(string)
	}

	return nil
}

// Validates if the monitoring component can be run. We assume that we are
// running in an Openshift environment with cluster monitoring enabled for our
// monitoring component to work
func (r *QuayRegistryReconciler) checkMonitoringAvailable(
	ctx context.Context, qctx *quaycontext.QuayRegistryContext,
) error {
	if len(r.WatchNamespace) > 0 {
		msg := "Monitoring is only supported in AllNamespaces mode. Disabling."
		r.Log.Info(msg)
		return fmt.Errorf(msg)
	}

	var serviceMonitors prometheusv1.ServiceMonitorList
	if err := r.Client.List(ctx, &serviceMonitors); err != nil {
		r.Log.Info("Unable to find ServiceMonitor CRD. Monitoring component disabled")
		return err
	}
	r.Log.Info("cluster supports `ServiceMonitor` API")

	var prometheusRules prometheusv1.PrometheusRuleList
	if err := r.Client.List(ctx, &prometheusRules); err != nil {
		r.Log.Info("Unable to find PrometheusRule CRD. Monitoring component disabled")
		return err
	}
	r.Log.Info("cluster supports `PrometheusRules` API")

	namespaceKey := types.NamespacedName{
		Name: grafanaDashboardConfigNamespace,
	}

	var grafanaDashboardNamespace corev1.Namespace
	if err := r.Client.Get(ctx, namespaceKey, &grafanaDashboardNamespace); err != nil {
		return fmt.Errorf("unable to get grafana namespace: %s", err)
	}

	r.Log.Info(grafanaDashboardConfigNamespace + " found")
	qctx.SupportsMonitoring = true
	return nil
}

// configEditorCredentialsSecretFrom returns the name of the secret that contains the
// credentials for the config editor. If the secret does not exist among the provided
// list an empty string is returned instead.
func configEditorCredentialsSecretFrom(objs []client.Object) string {
	for _, obj := range objs {
		if !strings.Contains(obj.GetName(), "quay-config-editor-credentials") {
			continue
		}

		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Version != "v1" {
			continue
		}
		if gvk.Kind != "Secret" {
			continue
		}

		return obj.GetName()
	}
	return ""
}

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
		err := pem.Encode(&b, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
		if err != nil {
			return nil, err
		}
	}

	return b.Bytes(), nil
}
