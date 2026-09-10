package controllers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/kustomize"
	"github.com/quay/quay-operator/pkg/middleware"
)

const (
	readOnlyMinimumQuayVersion = "3.19.0"
	readOnlyService            = "quay"

	readOnlyOriginalMinReplicasAnnotation = "quay.redhat.com/readonly-original-min-replicas"
	readOnlyOriginalMaxReplicasAnnotation = "quay.redhat.com/readonly-original-max-replicas"
)

var readOnlyHTTPClient = &http.Client{Timeout: 5 * time.Second}

type readOnlyIntent struct {
	Enabled              bool
	Phase                v1.ReadOnlyPhase
	MountServiceKey      bool
	RenderReadOnlyConfig bool
	PinHPAs              bool
	UseRecreateStrategy  bool
	DeferUpgrade         bool
	FreezeImages         bool
}

type readOnlyDecision struct {
	Stop   bool
	Result ctrl.Result
	Err    error
}

func (r *QuayRegistryReconciler) prepareReadOnlyLifecycle(
	ctx context.Context,
	quay *v1.QuayRegistry,
	qctx *quaycontext.QuayRegistryContext,
	usercfg map[string]interface{},
	cbundle *corev1.Secret,
	log logr.Logger,
) (readOnlyIntent, readOnlyDecision) {
	phase := quay.Status.ReadOnlyPhase
	requested := quay.Spec.ReadOnly != nil && *quay.Spec.ReadOnly
	exitRequested := phase != v1.ReadOnlyPhaseNormal && !requested

	if phase == v1.ReadOnlyPhaseNormal && !requested {
		if quay.Status.ReadOnlyUnsupportedImage != "" {
			quay.Status.ReadOnlyUnsupportedImage = ""
			return readOnlyIntent{}, r.readOnlyStatusDecision(ctx, quay)
		}
		if manualReadOnlyConfigured(usercfg, cbundle, quay) {
			qctx.ReadOnlyDeferUpgrade = true
			if err := r.collectReadOnlyFrozenImages(ctx, quay, qctx, false); err != nil {
				log.Error(err, "could not collect read-only image freeze state for manual read-only config")
			}
		}
		return readOnlyIntent{}, readOnlyDecision{}
	}

	if requested && phase == v1.ReadOnlyPhaseNormal {
		if blocked, decision := r.readOnlyCompatibilityBlocked(ctx, quay); blocked || decision.Stop {
			return readOnlyIntent{}, decision
		}
		if conflict := readOnlyOperatorConfigConflict(cbundle); conflict != "" {
			msg := fmt.Sprintf("operator-managed read-only is blocked by lifecycle config in %s", conflict)
			return readOnlyIntent{}, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonManualMigrationRequired, msg, false)
		}
		if conflict := readOnlyOverrideConflict(quay); conflict != "" {
			return readOnlyIntent{}, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonOverrideConflict, conflict, false)
		}

		kid, stop, err := r.ensureReadOnlyServiceKeySecret(ctx, quay, log)
		if err != nil {
			return readOnlyIntent{}, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDeferred, err.Error(), true)
		}
		if stop {
			return readOnlyIntent{}, readOnlyDecision{Stop: true, Result: r.Requeue, Err: nil}
		}
		quay.Status.ReadOnlyPhase = v1.ReadOnlyPhasePreparingKey
		quay.Status.ReadOnlyKeyID = kid
		return readOnlyIntent{}, r.readOnlyConditionDecision(
			ctx,
			quay,
			metav1.ConditionUnknown,
			v1.ConditionReasonReadOnlyTransitioning,
			"preparing operator-managed read-only service key",
			true,
		)
	}

	if exitRequested && phase != v1.ReadOnlyPhaseExitingReadOnly {
		quay.Status.ReadOnlyPhase = v1.ReadOnlyPhaseExitingReadOnly
		return readOnlyIntent{}, r.readOnlyConditionDecision(
			ctx,
			quay,
			metav1.ConditionUnknown,
			v1.ConditionReasonReadOnlyTransitioning,
			"exiting operator-managed read-only mode",
			true,
		)
	}

	intent := readOnlyIntent{Enabled: phase != v1.ReadOnlyPhaseNormal, Phase: phase}
	switch phase {
	case v1.ReadOnlyPhasePreparingKey:
		kid, stop, err := r.ensureReadOnlyServiceKeySecret(ctx, quay, log)
		if err != nil {
			return intent, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDeferred, err.Error(), true)
		}
		if stop {
			return intent, readOnlyDecision{Stop: true, Result: r.Requeue}
		}
		quay.Status.ReadOnlyKeyID = kid
		intent.MountServiceKey = true
		intent.DeferUpgrade = true
		intent.FreezeImages = true
	case v1.ReadOnlyPhaseEnteringReadOnly:
		if !r.prepareTransitionHPAs(ctx, quay, qctx, phase) {
			return intent, readOnlyDecision{Stop: true, Result: r.Requeue}
		}
		intent.MountServiceKey = true
		intent.RenderReadOnlyConfig = true
		intent.PinHPAs = true
		intent.UseRecreateStrategy = true
		intent.DeferUpgrade = true
		intent.FreezeImages = true
	case v1.ReadOnlyPhaseReadOnly:
		intent.MountServiceKey = true
		intent.RenderReadOnlyConfig = true
		intent.DeferUpgrade = true
		intent.FreezeImages = true
	case v1.ReadOnlyPhaseExitingReadOnly:
		if !r.prepareTransitionHPAs(ctx, quay, qctx, phase) {
			return intent, readOnlyDecision{Stop: true, Result: r.Requeue}
		}
		intent.PinHPAs = true
		intent.UseRecreateStrategy = true
		intent.DeferUpgrade = true
		intent.FreezeImages = true
	default:
		return intent, readOnlyDecision{}
	}

	if err := r.applyReadOnlyIntent(ctx, quay, qctx, intent); err != nil {
		reason := v1.ConditionReasonReadOnlyDeferred
		if phase == v1.ReadOnlyPhaseReadOnly {
			reason = v1.ConditionReasonReadOnlyDegraded
		}
		return intent, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, reason, err.Error(), true)
	}
	return intent, readOnlyDecision{}
}

func (r *QuayRegistryReconciler) observeReadOnlyLifecycle(
	ctx context.Context,
	quay *v1.QuayRegistry,
	qctx *quaycontext.QuayRegistryContext,
	intent readOnlyIntent,
	cbundle *corev1.Secret,
	log logr.Logger,
) readOnlyDecision {
	_ = cbundle
	if !intent.Enabled {
		return readOnlyDecision{}
	}

	switch intent.Phase {
	case v1.ReadOnlyPhasePreparingKey:
		return r.observePreparingKey(ctx, quay, log)
	case v1.ReadOnlyPhaseEnteringReadOnly:
		return r.observeEnteringReadOnly(ctx, quay, qctx, log)
	case v1.ReadOnlyPhaseReadOnly:
		if err := r.verifyReadOnlyKey(ctx, quay); err != nil {
			return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDegraded, err.Error(), true)
		}
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionTrue, v1.ConditionReasonReadOnlyActive, readOnlyActiveConditionMessage(quay, qctx), false)
	case v1.ReadOnlyPhaseExitingReadOnly:
		return r.observeExitingReadOnly(ctx, quay, qctx, log)
	default:
		return readOnlyDecision{}
	}
}

func (r *QuayRegistryReconciler) observePreparingKey(ctx context.Context, quay *v1.QuayRegistry, log logr.Logger) readOnlyDecision {
	rolledOut, err := r.readOnlyDeploymentsRolledOut(ctx, quay)
	if err != nil {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDeferred, err.Error(), true)
	}
	if !rolledOut {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionUnknown, v1.ConditionReasonReadOnlyTransitioning, "waiting for read-only service key mount rollout", true)
	}
	if err := r.verifyReadOnlyKey(ctx, quay); err != nil {
		if isReadOnlyUnsupportedError(err) {
			log.Info("read-only keyserver endpoint is unsupported by the current image", "error", err.Error())
			quay.Status.ReadOnlyUnsupportedImage = r.currentQuayImage(ctx, quay)
			quay.Status.ReadOnlyPhase = v1.ReadOnlyPhaseExitingReadOnly
			return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonUnsupportedVersion, err.Error(), true)
		}
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDeferred, err.Error(), true)
	}

	quay.Status.ReadOnlyPhase = v1.ReadOnlyPhaseEnteringReadOnly
	return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionUnknown, v1.ConditionReasonReadOnlyTransitioning, "entering operator-managed read-only mode", true)
}

func (r *QuayRegistryReconciler) observeEnteringReadOnly(
	ctx context.Context,
	quay *v1.QuayRegistry,
	qctx *quaycontext.QuayRegistryContext,
	log logr.Logger,
) readOnlyDecision {
	rolledOut, err := r.readOnlyDeploymentsRolledOut(ctx, quay)
	if err != nil {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDeferred, err.Error(), true)
	}
	if !rolledOut {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionUnknown, v1.ConditionReasonReadOnlyTransitioning, "waiting for read-only rollout", true)
	}
	if err := r.verifyReadOnlyKey(ctx, quay); err != nil {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDeferred, err.Error(), true)
	}
	if restored, err := r.restoreReadOnlyHPAs(ctx, quay, qctx); err != nil {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDeferred, err.Error(), true)
	} else if !restored {
		log.Info("restored read-only transition HPA pins, waiting for next reconcile")
		return readOnlyDecision{Stop: true, Result: r.Requeue}
	}

	quay.Status.ReadOnlyPhase = v1.ReadOnlyPhaseReadOnly
	return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionTrue, v1.ConditionReasonReadOnlyActive, readOnlyActiveConditionMessage(quay, qctx), true)
}

func (r *QuayRegistryReconciler) observeExitingReadOnly(
	ctx context.Context,
	quay *v1.QuayRegistry,
	qctx *quaycontext.QuayRegistryContext,
	log logr.Logger,
) readOnlyDecision {
	rolledOut, err := r.readOnlyDeploymentsRolledOut(ctx, quay)
	if err != nil {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDegraded, err.Error(), true)
	}
	if !rolledOut {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionUnknown, v1.ConditionReasonReadOnlyTransitioning, "waiting for normal no-mount rollout", true)
	}
	if restored, err := r.restoreReadOnlyHPAs(ctx, quay, qctx); err != nil {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDegraded, err.Error(), true)
	} else if !restored {
		log.Info("restored read-only exit HPA pins, waiting for next reconcile")
		return readOnlyDecision{Stop: true, Result: r.Requeue}
	}
	if err := r.deleteReadOnlySecret(ctx, quay); err != nil {
		return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDegraded, err.Error(), true)
	}

	quay.Status.ReadOnlyPhase = v1.ReadOnlyPhaseNormal
	quay.Status.ReadOnlyKeyID = ""
	return r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDisabled, "operator-managed read-only mode is disabled", true)
}

func (r *QuayRegistryReconciler) applyReadOnlyIntent(
	ctx context.Context,
	quay *v1.QuayRegistry,
	qctx *quaycontext.QuayRegistryContext,
	intent readOnlyIntent,
) error {
	qctx.ReadOnlyPhase = string(intent.Phase)
	qctx.ReadOnlyKeyID = quay.Status.ReadOnlyKeyID
	qctx.ReadOnlySecretName = readOnlySecretName(quay)
	qctx.ReadOnlyMountEnabled = intent.MountServiceKey
	qctx.ReadOnlyRegistryState = intent.RenderReadOnlyConfig
	qctx.ReadOnlyDeferUpgrade = intent.DeferUpgrade
	if intent.FreezeImages {
		return r.collectReadOnlyFrozenImages(ctx, quay, qctx, intent.Phase != v1.ReadOnlyPhasePreparingKey)
	}
	return nil
}

func (r *QuayRegistryReconciler) readOnlyCompatibilityBlocked(ctx context.Context, quay *v1.QuayRegistry) (bool, readOnlyDecision) {
	if quay.Status.ReadOnlyUnsupportedImage != "" {
		currentImage := r.currentQuayImage(ctx, quay)
		if currentImage == "" || currentImage == quay.Status.ReadOnlyUnsupportedImage {
			msg := fmt.Sprintf("Quay image %q does not expose required read-only endpoints", quay.Status.ReadOnlyUnsupportedImage)
			return true, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonUnsupportedVersion, msg, false)
		}
		quay.Status.ReadOnlyUnsupportedImage = ""
		return true, r.readOnlyStatusDecision(ctx, quay)
	}

	if quay.Status.CurrentVersion == "" {
		msg := fmt.Sprintf("waiting for status.currentVersion before starting read-only; required version is >= %s", readOnlyMinimumQuayVersion)
		return true, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonReadOnlyDeferred, msg, false)
	}
	currentVersion, err := semver.NewVersion(string(quay.Status.CurrentVersion))
	if err != nil {
		msg := fmt.Sprintf("status.currentVersion must be a valid semver value greater than or equal to %s", readOnlyMinimumQuayVersion)
		return true, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonUnsupportedVersion, msg, false)
	}
	minVersion := semver.MustParse(readOnlyMinimumQuayVersion)
	if currentVersion.LessThan(minVersion) {
		msg := fmt.Sprintf("status.currentVersion %s is less than required read-only version %s", currentVersion, minVersion)
		return true, r.readOnlyConditionDecision(ctx, quay, metav1.ConditionFalse, v1.ConditionReasonUnsupportedVersion, msg, false)
	}
	return false, readOnlyDecision{}
}

func (r *QuayRegistryReconciler) readOnlyConditionDecision(
	ctx context.Context,
	quay *v1.QuayRegistry,
	status metav1.ConditionStatus,
	reason v1.ConditionReason,
	msg string,
	stop bool,
) readOnlyDecision {
	if err := r.updateReadOnlyCondition(ctx, quay, status, reason, msg); err != nil {
		return readOnlyDecision{Stop: true, Result: r.Requeue, Err: err}
	}
	return readOnlyDecision{Stop: stop, Result: r.Requeue}
}

func (r *QuayRegistryReconciler) readOnlyStatusDecision(ctx context.Context, quay *v1.QuayRegistry) readOnlyDecision {
	quay.Status.LastUpdate = time.Now().UTC().String()
	if err := r.Status().Update(ctx, quay); err != nil {
		return readOnlyDecision{Stop: true, Result: r.Requeue, Err: err}
	}
	return readOnlyDecision{Stop: true, Result: r.Requeue}
}

func readOnlyActiveConditionMessage(quay *v1.QuayRegistry, qctx *quaycontext.QuayRegistryContext) string {
	const activeMessage = "operator-managed read-only mode is active"
	if qctx == nil ||
		!qctx.ReadOnlyDeferUpgrade ||
		quay.Status.CurrentVersion == "" ||
		v1.QuayVersionCurrent == "" ||
		quay.Status.CurrentVersion == v1.QuayVersionCurrent {
		return activeMessage
	}
	return fmt.Sprintf(
		"operator upgrade from %s to %s deferred while operator-managed read-only mode is active",
		quay.Status.CurrentVersion,
		v1.QuayVersionCurrent,
	)
}

func readOnlySecretName(quay *v1.QuayRegistry) string {
	return quay.GetName() + "-readonly-service-key"
}

func (r *QuayRegistryReconciler) ensureReadOnlyServiceKeySecret(
	ctx context.Context,
	quay *v1.QuayRegistry,
	log logr.Logger,
) (string, bool, error) {
	var secret corev1.Secret
	key := types.NamespacedName{Name: readOnlySecretName(quay), Namespace: quay.GetNamespace()}
	if err := r.Get(ctx, key, &secret); err != nil {
		if !errors.IsNotFound(err) {
			return "", false, err
		}
		generated, kid, err := generateReadOnlySecret(quay)
		if err != nil {
			return "", false, err
		}
		if err := controllerutil.SetControllerReference(quay, generated, r.Scheme); err != nil {
			return "", false, err
		}
		if err := r.Create(ctx, generated); err != nil {
			return "", false, err
		}
		log.Info("created read-only service key Secret", "secret", generated.Name, "kid", kid)
		return kid, false, nil
	}

	if !ownedByQuay(&secret, quay) {
		return "", false, fmt.Errorf("read-only service key Secret %q already exists and is not owned by this QuayRegistry", secret.Name)
	}

	kid, err := validateReadOnlySecret(&secret)
	if err != nil {
		if quay.Status.ReadOnlyPhase == v1.ReadOnlyPhaseReadOnly {
			return "", false, fmt.Errorf("read-only service key Secret %q is invalid during active read-only: %w", secret.Name, err)
		}
		if quay.Status.ReadOnlyPhase == v1.ReadOnlyPhaseNormal {
			if err := r.Delete(ctx, &secret); err != nil {
				return "", false, err
			}
			log.Info("deleted invalid read-only service key Secret before mounting", "secret", secret.Name)
			return "", true, nil
		}
		return "", false, fmt.Errorf("read-only service key Secret %q is invalid after mounting; exit read-only before recreating it: %w", secret.Name, err)
	}

	if quay.Status.ReadOnlyKeyID != "" && quay.Status.ReadOnlyKeyID != kid {
		return "", false, fmt.Errorf("read-only service key Secret kid %q does not match status.readOnlyKeyID %q", kid, quay.Status.ReadOnlyKeyID)
	}
	return kid, false, nil
}

func generateReadOnlySecret(quay *v1.QuayRegistry) (*corev1.Secret, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	kid := jwkThumbprint(&key.PublicKey)
	immutable := true
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      readOnlySecretName(quay),
			Namespace: quay.GetNamespace(),
		},
		Immutable: &immutable,
		Type:      corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			kustomize.ReadOnlyKIDKey: []byte(kid),
			kustomize.ReadOnlyPEMKey: pemBytes,
		},
	}, kid, nil
}

func validateReadOnlySecret(secret *corev1.Secret) (string, error) {
	kidBytes, ok := secret.Data[kustomize.ReadOnlyKIDKey]
	if !ok || len(kidBytes) == 0 {
		return "", fmt.Errorf("missing %s", kustomize.ReadOnlyKIDKey)
	}
	pemBytes, ok := secret.Data[kustomize.ReadOnlyPEMKey]
	if !ok || len(pemBytes) == 0 {
		return "", fmt.Errorf("missing %s", kustomize.ReadOnlyPEMKey)
	}
	key, err := parseReadOnlyPrivateKey(pemBytes)
	if err != nil {
		return "", err
	}
	kid := strings.TrimSpace(string(kidBytes))
	if expected := jwkThumbprint(&key.PublicKey); kid != expected {
		return "", fmt.Errorf("kid %q does not match private key thumbprint %q", kid, expected)
	}
	return kid, nil
}

func parseReadOnlyPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("read-only private key is not PEM encoded")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("read-only private key is not RSA")
	}
	return rsaKey, nil
}

func jwkThumbprint(pub *rsa.PublicKey) string {
	payload := fmt.Sprintf(
		`{"e":"%s","kty":"RSA","n":"%s"}`,
		base64.RawURLEncoding.EncodeToString(bigIntBytes(pub.E)),
		base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
	)
	sum := sha256.Sum256([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func bigIntBytes(v int) []byte {
	if v == 0 {
		return []byte{0}
	}
	var out []byte
	for v > 0 {
		out = append([]byte{byte(v & 0xff)}, out...)
		v >>= 8
	}
	return out
}

func ownedByQuay(obj client.Object, quay *v1.QuayRegistry) bool {
	if quay.UID == "" {
		return false
	}
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == "QuayRegistry" && ref.UID == quay.UID {
			return true
		}
	}
	return false
}

func (r *QuayRegistryReconciler) deleteReadOnlySecret(ctx context.Context, quay *v1.QuayRegistry) error {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: readOnlySecretName(quay), Namespace: quay.GetNamespace()}, &secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if !ownedByQuay(&secret, quay) {
		return fmt.Errorf("read-only service key Secret %q exists but is not owned by this QuayRegistry", secret.Name)
	}
	if err := r.Delete(ctx, &secret); err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func readOnlyOperatorConfigConflict(cbundle *corev1.Secret) string {
	for name, data := range cbundle.Data {
		if name != "config.yaml" && !strings.Contains(name, ".config.yaml") {
			continue
		}
		var cfg map[string]interface{}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return name
		}
		for _, key := range readOnlyLifecycleConfigKeys() {
			if _, ok := cfg[key]; ok {
				return name
			}
		}
	}
	return ""
}

func readOnlyOverrideConflict(quay *v1.QuayRegistry) string {
	for _, kind := range []v1.ComponentKind{v1.ComponentQuay, v1.ComponentMirror} {
		for _, env := range v1.GetEnvOverrideForComponent(quay, kind) {
			if env.Name != "QUAY_OVERRIDE_CONFIG" {
				continue
			}
			if env.ValueFrom != nil {
				return fmt.Sprintf("%s override QUAY_OVERRIDE_CONFIG uses valueFrom, which cannot be validated for operator-managed read-only", kind)
			}
			var cfg map[string]interface{}
			if err := json.Unmarshal([]byte(env.Value), &cfg); err != nil {
				return fmt.Sprintf("%s override QUAY_OVERRIDE_CONFIG is not valid JSON", kind)
			}
			for _, key := range readOnlyLifecycleConfigKeys() {
				if _, ok := cfg[key]; ok {
					return fmt.Sprintf("%s override QUAY_OVERRIDE_CONFIG contains read-only lifecycle key %s", kind, key)
				}
			}
		}
	}
	return ""
}

func manualReadOnlyConfigured(usercfg map[string]interface{}, cbundle *corev1.Secret, quay *v1.QuayRegistry) bool {
	if value, ok := usercfg["REGISTRY_STATE"].(string); ok && value == kustomize.ReadOnlyRegistryState {
		return true
	}
	flattened, err := middleware.FlattenSecret(cbundle)
	if err == nil {
		var cfg map[string]interface{}
		if err := yaml.Unmarshal(flattened.Data["config.yaml"], &cfg); err == nil {
			if value, ok := cfg["REGISTRY_STATE"].(string); ok && value == kustomize.ReadOnlyRegistryState {
				return true
			}
		}
	}
	for _, kind := range []v1.ComponentKind{v1.ComponentQuay, v1.ComponentMirror} {
		for _, env := range v1.GetEnvOverrideForComponent(quay, kind) {
			if env.Name != "QUAY_OVERRIDE_CONFIG" || env.Value == "" {
				continue
			}
			var cfg map[string]interface{}
			if err := json.Unmarshal([]byte(env.Value), &cfg); err != nil {
				continue
			}
			if value, ok := cfg["REGISTRY_STATE"].(string); ok && value == kustomize.ReadOnlyRegistryState {
				return true
			}
		}
	}
	return false
}

func readOnlyLifecycleConfigKeys() []string {
	return []string{
		"REGISTRY_STATE",
		"INSTANCE_SERVICE_KEY_KID_LOCATION",
		"INSTANCE_SERVICE_KEY_LOCATION",
		"INSTANCE_SERVICE_KEY_IMPORT_FROM_FILES",
		"INSTANCE_SERVICE_KEY_EXPIRATION",
	}
}

func (r *QuayRegistryReconciler) prepareTransitionHPAs(
	ctx context.Context,
	quay *v1.QuayRegistry,
	qctx *quaycontext.QuayRegistryContext,
	phase v1.ReadOnlyPhase,
) bool {
	if !v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentHPA) {
		return true
	}

	targets := []string{quay.GetName() + "-quay-app"}
	if v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentMirror) {
		targets = append(targets, quay.GetName()+"-quay-mirror")
	}

	allPinned := true
	if qctx.ReadOnlyHPAPins == nil {
		qctx.ReadOnlyHPAPins = map[string]int32{}
	}
	for _, target := range targets {
		pinned, replicas, err := r.ensureReadOnlyHPAPin(ctx, quay, target)
		if err != nil {
			status := metav1.ConditionFalse
			reason := v1.ConditionReasonReadOnlyDeferred
			if phase == v1.ReadOnlyPhaseReadOnly || phase == v1.ReadOnlyPhaseExitingReadOnly {
				reason = v1.ConditionReasonReadOnlyDegraded
			}
			_ = r.updateReadOnlyCondition(ctx, quay, status, reason, err.Error())
			return false
		}
		if !pinned {
			allPinned = false
		}
		qctx.ReadOnlyHPAPins[target] = replicas
	}
	return allPinned
}

func (r *QuayRegistryReconciler) ensureReadOnlyHPAPin(
	ctx context.Context,
	quay *v1.QuayRegistry,
	target string,
) (bool, int32, error) {
	var dep appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: target, Namespace: quay.GetNamespace()}, &dep); err != nil {
		return false, 0, fmt.Errorf("cannot pin HPA for %s: %w", target, err)
	}
	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}
	if replicas == 0 {
		return false, 0, fmt.Errorf("cannot enter read-only transition while %s has zero replicas", target)
	}

	hpa, err := r.hpaForTarget(ctx, quay, target)
	if err != nil {
		return false, 0, err
	}
	if hpa.Annotations == nil {
		hpa.Annotations = map[string]string{}
	}
	if _, ok := hpa.Annotations[readOnlyOriginalMaxReplicasAnnotation]; !ok {
		hpa.Annotations[readOnlyOriginalMaxReplicasAnnotation] = strconv.Itoa(int(hpa.Spec.MaxReplicas))
	}
	if _, ok := hpa.Annotations[readOnlyOriginalMinReplicasAnnotation]; !ok {
		if hpa.Spec.MinReplicas != nil {
			hpa.Annotations[readOnlyOriginalMinReplicasAnnotation] = strconv.Itoa(int(*hpa.Spec.MinReplicas))
		} else {
			hpa.Annotations[readOnlyOriginalMinReplicasAnnotation] = ""
		}
	}

	pinned := hpa.Spec.MinReplicas != nil &&
		*hpa.Spec.MinReplicas == replicas &&
		hpa.Spec.MaxReplicas == replicas
	if pinned {
		return true, replicas, nil
	}

	hpa.Spec.MinReplicas = &replicas
	hpa.Spec.MaxReplicas = replicas
	if err := r.Update(ctx, hpa); err != nil {
		return false, 0, err
	}
	return false, replicas, nil
}

func (r *QuayRegistryReconciler) hpaForTarget(ctx context.Context, quay *v1.QuayRegistry, target string) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	var hpas autoscalingv2.HorizontalPodAutoscalerList
	if err := r.List(ctx, &hpas, client.InNamespace(quay.GetNamespace())); err != nil {
		return nil, err
	}
	for i := range hpas.Items {
		if hpas.Items[i].Spec.ScaleTargetRef.Name == target {
			return &hpas.Items[i], nil
		}
	}
	return nil, fmt.Errorf("managed HPA targeting %s not found", target)
}

func (r *QuayRegistryReconciler) restoreReadOnlyHPAs(
	ctx context.Context,
	quay *v1.QuayRegistry,
	qctx *quaycontext.QuayRegistryContext,
) (bool, error) {
	_ = qctx
	if !v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentHPA) {
		return true, nil
	}
	var hpas autoscalingv2.HorizontalPodAutoscalerList
	if err := r.List(ctx, &hpas, client.InNamespace(quay.GetNamespace())); err != nil {
		return false, err
	}

	allRestored := true
	for i := range hpas.Items {
		hpa := &hpas.Items[i]
		if hpa.Annotations == nil {
			continue
		}
		maxRaw, hasMax := hpa.Annotations[readOnlyOriginalMaxReplicasAnnotation]
		minRaw, hasMin := hpa.Annotations[readOnlyOriginalMinReplicasAnnotation]
		if !hasMax && !hasMin {
			continue
		}
		if hasMax {
			max, err := strconv.Atoi(maxRaw)
			if err != nil {
				return false, err
			}
			hpa.Spec.MaxReplicas = int32(max)
		}
		if hasMin {
			if minRaw == "" {
				hpa.Spec.MinReplicas = nil
			} else {
				min, err := strconv.Atoi(minRaw)
				if err != nil {
					return false, err
				}
				min32 := int32(min)
				hpa.Spec.MinReplicas = &min32
			}
		}
		delete(hpa.Annotations, readOnlyOriginalMaxReplicasAnnotation)
		delete(hpa.Annotations, readOnlyOriginalMinReplicasAnnotation)
		if err := r.Update(ctx, hpa); err != nil {
			return false, err
		}
		allRestored = false
	}
	return allRestored, nil
}

func (r *QuayRegistryReconciler) collectReadOnlyFrozenImages(
	ctx context.Context,
	quay *v1.QuayRegistry,
	qctx *quaycontext.QuayRegistryContext,
	require bool,
) error {
	if qctx.ReadOnlyFrozenImages == nil {
		qctx.ReadOnlyFrozenImages = map[string]string{}
	}
	targets := []string{quay.GetName() + "-quay-app"}
	if v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentMirror) {
		targets = append(targets, quay.GetName()+"-quay-mirror")
	}
	for _, target := range targets {
		var dep appsv1.Deployment
		if err := r.Get(ctx, types.NamespacedName{Name: target, Namespace: quay.GetNamespace()}, &dep); err != nil {
			if !require && errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("cannot freeze images for %s: %w", target, err)
		}
		for _, container := range dep.Spec.Template.Spec.InitContainers {
			qctx.ReadOnlyFrozenImages[readOnlyFrozenImageKey(dep.Name, "initContainers", container.Name)] = container.Image
		}
		for _, container := range dep.Spec.Template.Spec.Containers {
			qctx.ReadOnlyFrozenImages[readOnlyFrozenImageKey(dep.Name, "containers", container.Name)] = container.Image
		}
	}
	return nil
}

func readOnlyFrozenImageKey(deploymentName, containerGroup, containerName string) string {
	return deploymentName + "/" + containerGroup + "/" + containerName
}

func (r *QuayRegistryReconciler) readOnlyDeploymentsRolledOut(ctx context.Context, quay *v1.QuayRegistry) (bool, error) {
	targets := []string{quay.GetName() + "-quay-app"}
	if v1.ComponentIsManaged(quay.Spec.Components, v1.ComponentMirror) {
		targets = append(targets, quay.GetName()+"-quay-mirror")
	}
	for _, target := range targets {
		rolledOut, err := r.deploymentRolledOut(ctx, quay, target)
		if err != nil || !rolledOut {
			return rolledOut, err
		}
	}
	return true, nil
}

func (r *QuayRegistryReconciler) deploymentRolledOut(ctx context.Context, quay *v1.QuayRegistry, name string) (bool, error) {
	var deployment appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: quay.GetNamespace()}, &deployment); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false, nil
	}
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	return deployment.Status.UpdatedReplicas == desired &&
		deployment.Status.AvailableReplicas == desired &&
		deployment.Status.Replicas == desired, nil
}

func (r *QuayRegistryReconciler) currentQuayImage(ctx context.Context, quay *v1.QuayRegistry) string {
	var dep appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: quay.GetName() + "-quay-app", Namespace: quay.GetNamespace()}, &dep); err != nil {
		return ""
	}
	for _, container := range dep.Spec.Template.Spec.Containers {
		if container.Name == "quay-app" {
			return container.Image
		}
	}
	if len(dep.Spec.Template.Spec.Containers) > 0 {
		return dep.Spec.Template.Spec.Containers[0].Image
	}
	return ""
}

type readOnlyKeyStatus struct {
	KID             string `json:"kid"`
	Service         string `json:"service"`
	OperatorManaged bool   `json:"operator_managed"`
	ExpirationDate  string `json:"expiration_date"`
	Approved        *bool  `json:"approved,omitempty"`
}

type readOnlyPublicJWK struct {
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type readOnlyVerificationError struct {
	message     string
	unsupported bool
}

func (e readOnlyVerificationError) Error() string { return e.message }

func isReadOnlyUnsupportedError(err error) bool {
	if e, ok := err.(readOnlyVerificationError); ok {
		return e.unsupported
	}
	return false
}

func (r *QuayRegistryReconciler) verifyReadOnlyKey(ctx context.Context, quay *v1.QuayRegistry) error {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: readOnlySecretName(quay), Namespace: quay.GetNamespace()}, &secret); err != nil {
		return fmt.Errorf("read-only service key Secret unavailable: %w", err)
	}
	kid, err := validateReadOnlySecret(&secret)
	if err != nil {
		return err
	}
	if quay.Status.ReadOnlyKeyID != "" && quay.Status.ReadOnlyKeyID != kid {
		return fmt.Errorf("read-only Secret kid %q does not match status.readOnlyKeyID %q", kid, quay.Status.ReadOnlyKeyID)
	}

	statusURL := readOnlyKeyURL(quay, kid) + "/status"
	statusBody, err := readOnlyHTTPGet(ctx, statusURL)
	if err != nil {
		return err
	}
	var status readOnlyKeyStatus
	if err := json.Unmarshal(statusBody, &status); err != nil {
		return readOnlyVerificationError{message: fmt.Sprintf("read-only key status response is malformed: %s", err), unsupported: true}
	}
	if status.KID == "" || status.Service == "" || status.ExpirationDate == "" {
		return readOnlyVerificationError{message: "read-only key status response is missing required fields", unsupported: true}
	}
	if status.KID != kid || status.Service != readOnlyService || !status.OperatorManaged {
		return fmt.Errorf("read-only key status has not converged for kid %q", kid)
	}
	expiration, err := time.Parse(time.RFC3339, status.ExpirationDate)
	if err != nil {
		return readOnlyVerificationError{message: fmt.Sprintf("read-only key expiration_date is invalid: %s", err), unsupported: true}
	}
	if !expiration.After(time.Now()) {
		return fmt.Errorf("read-only service key %q is expired", kid)
	}
	if status.Approved != nil && !*status.Approved {
		return fmt.Errorf("read-only service key %q is not approved", kid)
	}

	rawBody, err := readOnlyHTTPGet(ctx, readOnlyKeyURL(quay, kid))
	if err != nil {
		return err
	}
	var raw readOnlyPublicJWK
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return readOnlyVerificationError{message: fmt.Sprintf("read-only public JWK response is malformed: %s", err), unsupported: true}
	}
	if raw.Kty != "RSA" || raw.N == "" || raw.E == "" {
		return readOnlyVerificationError{message: "read-only public JWK response is missing RSA public key fields", unsupported: true}
	}
	privateKey, err := parseReadOnlyPrivateKey(secret.Data[kustomize.ReadOnlyPEMKey])
	if err != nil {
		return err
	}
	expected := publicJWK(&privateKey.PublicKey)
	if raw.N != expected.N || raw.E != expected.E {
		return fmt.Errorf("read-only public JWK does not match mounted Secret key")
	}
	return nil
}

func readOnlyKeyURL(quay *v1.QuayRegistry, kid string) string {
	return fmt.Sprintf(
		"http://%s-quay-app.%s.svc:80/keys/services/%s/keys/%s",
		quay.GetName(),
		quay.GetNamespace(),
		readOnlyService,
		kid,
	)
}

func readOnlyHTTPGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := readOnlyHTTPClient.Do(req)
	if err != nil {
		return nil, readOnlyVerificationError{message: err.Error()}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return body, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNotImplemented:
		return nil, readOnlyVerificationError{message: fmt.Sprintf("read-only key endpoint returned %d", resp.StatusCode), unsupported: true}
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, readOnlyVerificationError{message: fmt.Sprintf("read-only key endpoint returned %d", resp.StatusCode)}
	default:
		return nil, readOnlyVerificationError{message: fmt.Sprintf("read-only key endpoint returned %d", resp.StatusCode)}
	}
}

func publicJWK(pub *rsa.PublicKey) readOnlyPublicJWK {
	return readOnlyPublicJWK{
		Kty: "RSA",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(bigIntBytes(pub.E)),
	}
}
