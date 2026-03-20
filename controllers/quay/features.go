package controllers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	err "errors"
	"fmt"
	"strings"

	routev1 "github.com/openshift/api/route/v1"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

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
	securityScannerV4PSK = "SECURITY_SCANNER_V4_PSK"

	clairDbUser   = "CLAIR_DB_USER"
	clairDbPw     = "CLAIR_DB_PASSWORD"
	clairDbRootPw = "CLAIR_DB_ROOT_PW"
	clairDbName   = "CLAIR_DB_NAME"
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
		qctx.SecurityScannerV4PSK = string(secret.Data[securityScannerV4PSK])
		qctx.ClairDbUser = string(secret.Data[clairDbUser])
		qctx.ClairDbPassword = string(secret.Data[clairDbPw])
		qctx.ClairDbRootPw = string(secret.Data[clairDbRootPw])
		qctx.ClairDbName = string(secret.Data[clairDbName])
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
	qctx.SecurityScannerV4PSK = string(secret.Data[securityScannerV4PSK])
	qctx.ClairDbUser = string(secret.Data[clairDbUser])
	qctx.ClairDbPassword = string(secret.Data[clairDbPw])
	qctx.ClairDbRootPw = string(secret.Data[clairDbRootPw])
	qctx.ClairDbName = string(secret.Data[clairDbName])
	return nil
}

// checkClusterCAHash populates the provided QuayRegistryContext with revision version of the cluster provided CA configmaps.
// We must track these revisions so that we can force a restart of the QuayRegistry pods when the CA configmaps are updated.
func (r *QuayRegistryReconciler) checkClusterCAHash(
	ctx context.Context, qctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry,
) error {
	hashConfigMapContents := func(data map[string]string, key string) string {
		certData, exists := data[key]
		if !exists {
			return ""
		}
		hash := sha256.Sum256([]byte(certData))
		hashStr := hex.EncodeToString(hash[:])
		return hashStr[len(hashStr)-8:]
	}

	// Get cluster-service-ca hash
	clusterServiceCAnsn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", quay.Name, v1.ClusterServiceCAName),
		Namespace: quay.Namespace,
	}
	var clusterServiceCA corev1.ConfigMap
	if err := r.Get(ctx, clusterServiceCAnsn, &clusterServiceCA); err == nil {
		qctx.ClusterServiceCAHash = hashConfigMapContents(clusterServiceCA.Data, "service-ca.crt")
		if currentHash, exists := clusterServiceCA.Annotations[v1.ClusterServiceCAName]; !exists || currentHash != qctx.ClusterServiceCAHash {
			r.Log.Info("Detected change in cluster-service-ca configmap, updating annotation to trigger restart")
			clusterServiceCA.Annotations[v1.ClusterServiceCAName] = qctx.ClusterServiceCAHash
			if err := r.Update(ctx, &clusterServiceCA); err != nil {
				r.Log.Error(err, "unable to update cluster-service-ca configmap annotations")
				return err
			}
		}
	}

	clusterTrustedCAnsn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", quay.Name, v1.ClusterTrustedCAName),
		Namespace: quay.Namespace,
	}
	var clusterTrustedCA corev1.ConfigMap
	if err := r.Get(ctx, clusterTrustedCAnsn, &clusterTrustedCA); err == nil {
		qctx.ClusterTrustedCAHash = hashConfigMapContents(clusterTrustedCA.Data, "ca-bundle.crt")
		if currentHash, exists := clusterTrustedCA.Annotations[v1.ClusterTrustedCAName]; !exists || currentHash != qctx.ClusterTrustedCAHash {
			r.Log.Info("Detected change in cluster-trusted-ca configmap, updating annotation to trigger restart")
			clusterTrustedCA.Annotations[v1.ClusterTrustedCAName] = qctx.ClusterTrustedCAHash
			if err := r.Update(ctx, &clusterTrustedCA); err != nil {
				r.Log.Error(err, "unable to update cluster-trusted-ca configmap annotations")
				return err
			}
		}
	}

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

var errRouteProbeInProgress = fmt.Errorf("route probe in progress, awaiting ingress status")

// parseServerHostname extracts the SERVER_HOSTNAME field from the config bundle
// and sets it on the QuayRegistryContext if present.
func parseServerHostname(
	qctx *quaycontext.QuayRegistryContext,
	bundle *corev1.Secret,
) error {
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
	return nil
}

// ensureRouteDiscovery discovers the cluster hostname and wildcard cert via a
// probe Route. Results are cached for the operator's lifetime. Returns
// errRouteProbeInProgress if the probe route exists but status.ingress is not
// yet populated (caller should requeue).
func (r *QuayRegistryReconciler) ensureRouteDiscovery(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
) error {
	// Fast path: hostname already cached from a previous reconcile.
	if cached := r.clusterHostname.Load(); cached != nil {
		qctx.SupportsRoutes = true
		qctx.ClusterHostname = *cached
		if cachedCert := r.clusterWildcardCert.Load(); cachedCert != nil {
			qctx.ClusterWildcardCert = *cachedCert
		}
		return nil
	}

	routeName := fmt.Sprintf("%s-test-route", quay.GetName())
	routeNSN := types.NamespacedName{
		Name:      routeName,
		Namespace: quay.GetNamespace(),
	}

	fakeRoute := v1.EnsureOwnerReference(
		quay,
		&routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      routeName,
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

	if err := r.Create(ctx, fakeRoute); err != nil && !errors.IsAlreadyExists(err) {
		r.Log.Info("failed to create probe route", "error", err)
		return nil
	}

	var rt routev1.Route
	if err := r.Get(ctx, routeNSN, &rt); err != nil {
		return fmt.Errorf("failed to get probe route: %w", err)
	}

	if len(rt.Status.Ingress) == 0 {
		r.Log.Info("waiting to detect routerCanonicalHostname")
		return errRouteProbeInProgress
	}

	// Extract and cache cluster hostname.
	prefix := fmt.Sprintf("router-%s.", rt.Status.Ingress[0].RouterName)
	cnname := rt.Status.Ingress[0].RouterCanonicalHostname
	hostname := strings.TrimPrefix(cnname, prefix)
	r.Log.Info("detected cluster hostname", "hostname", hostname)

	qctx.SupportsRoutes = true
	qctx.ClusterHostname = hostname
	r.clusterHostname.Store(&hostname)

	// Best-effort wildcard cert extraction.
	wildcard, err := getCertificatesPEM(rt.Spec.Host + ":443")
	if err != nil {
		r.Log.Info("failed to extract wildcard cert, continuing without it", "error", err)
	} else {
		qctx.ClusterWildcardCert = wildcard
		r.clusterWildcardCert.Store(&wildcard)
	}

	if err := r.Delete(ctx, &rt); err != nil && !errors.IsNotFound(err) {
		r.Log.Error(err, "failed to delete probe route")
	}

	return nil
}

// fillServerHostname derives ServerHostname from ClusterHostname if not
// explicitly set in the config bundle.
func fillServerHostname(
	qctx *quaycontext.QuayRegistryContext,
	quay *v1.QuayRegistry,
) {
	if qctx.ServerHostname != "" {
		return
	}
	if qctx.ClusterHostname == "" {
		return
	}
	qctx.ServerHostname = fmt.Sprintf(
		"%s-quay-%s.%s",
		quay.GetName(),
		quay.GetNamespace(),
		qctx.ClusterHostname,
	)
}

func (r *QuayRegistryReconciler) checkObjectBucketClaimsAvailable(
	ctx context.Context, qctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry,
) error {
	dstorensn := types.NamespacedName{
		Name:      fmt.Sprintf("%s-quay-datastore", quay.GetName()),
		Namespace: quay.GetNamespace(),
	}

	var claims unstructured.UnstructuredList
	claims.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "objectbucket.io",
		Version: "v1alpha1",
		Kind:    "ObjectBucketClaimList",
	})
	if err := r.List(ctx, &claims, client.InNamespace(quay.GetNamespace())); err != nil {
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
		if err := r.Get(ctx, dstorensn, &datastoreSecret); err != nil {
			r.Log.Error(err, "error retrieving secret, bucket claim not ready")
			return fmt.Errorf("awaiting for bucket claim to be processed")
		}

		var datastoreConfig corev1.ConfigMap
		if err := r.Get(ctx, dstorensn, &datastoreConfig); err != nil {
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
		msg := "monitoring is only supported in AllNamespaces mode"
		r.Log.Info(msg)
		return err.New(msg)
	}

	var serviceMonitors prometheusv1.ServiceMonitorList
	if err := r.List(ctx, &serviceMonitors); err != nil {
		r.Log.Info("Unable to find ServiceMonitor CRD. Monitoring component disabled")
		return err
	}
	r.Log.Info("cluster supports `ServiceMonitor` API")

	var prometheusRules prometheusv1.PrometheusRuleList
	if err := r.List(ctx, &prometheusRules); err != nil {
		r.Log.Info("Unable to find PrometheusRule CRD. Monitoring component disabled")
		return err
	}
	r.Log.Info("cluster supports `PrometheusRules` API")

	namespaceKey := types.NamespacedName{
		Name: grafanaDashboardConfigNamespace,
	}

	var grafanaDashboardNamespace corev1.Namespace
	if err := r.Get(ctx, namespaceKey, &grafanaDashboardNamespace); err != nil {
		return fmt.Errorf("unable to get grafana namespace: %s", err)
	}

	r.Log.Info(grafanaDashboardConfigNamespace + " found")
	qctx.SupportsMonitoring = true
	return nil
}

// checkPostgresVersion returns the image name used by the currently deployed postgres version
func (r *QuayRegistryReconciler) checkNeedsPostgresUpgradeForComponent(
	ctx context.Context, qctx *quaycontext.QuayRegistryContext, quay *v1.QuayRegistry, component v1.ComponentKind,
) (err error, scaledDown bool) {
	componentInfo := map[v1.ComponentKind]struct {
		deploymentSuffix string
		upgradeField     *bool
	}{
		v1.ComponentClairPostgres: {"clair-postgres", &qctx.NeedsClairPgUpgrade},
		v1.ComponentPostgres:      {"quay-database", &qctx.NeedsPgUpgrade},
	}

	info, ok := componentInfo[component]
	if !ok {
		return fmt.Errorf("invalid component kind: %s", component), false
	}

	deploymentName := fmt.Sprintf("%s-%s", quay.GetName(), info.deploymentSuffix)
	r.Log.Info(fmt.Sprintf("getting %s version", component))

	postgresDeployment := &appsv1.Deployment{}
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      deploymentName,
			Namespace: quay.GetNamespace(),
		},
		postgresDeployment,
	); err != nil {
		r.Log.Info(fmt.Sprintf("%s deployment not found, skipping", component))
		return nil, true
	}

	deployedImageName := postgresDeployment.Spec.Template.Spec.Containers[0].Image
	r.Log.Info(fmt.Sprintf("%s deployment found", component), "image", deployedImageName)

	expectedImage, err := kustomize.ComponentImageFor(component)
	if err != nil {
		r.Log.Error(err, "failed to get postgres image")
	}

	expectedName := expectedImage.NewName
	if expectedName == "" {
		expectedName = expectedImage.Name
	}

	currentName := extractImageName(deployedImageName)

	// Extract repository names by finding the last component after splitting by '/'
	// This handles cases where repository names contain slashes (e.g., "quay.io/org/repo")
	currentRepoName := currentName[strings.LastIndex(currentName, "/")+1:]
	expectedRepoName := expectedName[strings.LastIndex(expectedName, "/")+1:]

	if currentRepoName != expectedRepoName {
		r.Log.Info(fmt.Sprintf("%s needs to perform an upgrade, marking in context", component))
		*info.upgradeField = true
	} else {
		r.Log.Info(fmt.Sprintf("%s does not need to perform an upgrade", component))
		return nil, true
	}

	// at this point we have determined that these postgres deployments need to be upgraded and can set them to 0 replicas
	// so that the upgrade job can run with no interference
	r.Log.Info(fmt.Sprintf("scaling down %s deployment", component))
	postgresDeployment.Spec.Replicas = &[]int32{0}[0]
	postgresDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds = &[]int64{600}[0]
	if err := r.Update(ctx, postgresDeployment); err != nil {
		r.Log.Error(err, "unable to update postgres deployment replicas")
	}
	// now we wait to ensure that the deployment has scaled down before we proceed

	terminatingPods := []corev1.Pod{}
	podList := &corev1.PodList{}
	labelSelector, err := metav1.LabelSelectorAsSelector(postgresDeployment.Spec.Selector)
	if err != nil {
		r.Log.Error(err, "unable to get label selector for postgres deployment")
	}
	err = r.List(ctx, podList, &client.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		r.Log.Error(err, "unable to list pods for postgres deployment")
		return err, false
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			terminatingPods = append(terminatingPods, pod)
		}
	}

	if len(terminatingPods) > 0 {
		r.Log.Info(fmt.Sprintf("Found %d pods in terminating status", len(terminatingPods)))
		return nil, false
	}

	return nil, true
}

func extractImageName(imageName string) string {
	parts := strings.Split(imageName, "@")
	if len(parts) > 1 {
		return parts[0]
	}
	parts = strings.Split(imageName, ":")
	if len(parts) > 1 {
		return parts[0]
	}
	return imageName
}

// Taken from https://stackoverflow.com/questions/46735347/how-can-i-fetch-a-certificate-from-a-url
func getCertificatesPEM(address string) ([]byte, error) {
	conn, err := tls.Dial("tcp", address, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
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
