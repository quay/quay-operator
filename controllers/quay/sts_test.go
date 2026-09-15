package controllers

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
	"github.com/quay/quay-operator/pkg/credentialsrequest"
)

const (
	testSTSRoleARN = "arn:aws:iam::123456789012:role/quay-role"
	testSTSBucket  = "quay-bucket"
)

func testQuayRegistry() *v1.QuayRegistry {
	return &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-ns",
			UID:       types.UID("test-uid"),
		},
	}
}

func testSTSContext() *quaycontext.QuayRegistryContext {
	return &quaycontext.QuayRegistryContext{
		StorageSTSEnabled: true,
		STSRoleARN:        testSTSRoleARN,
		STSStorageBuckets: []string{testSTSBucket},
	}
}

func testCredentialsRequest(t *testing.T, quay *v1.QuayRegistry) *unstructured.Unstructured {
	t.Helper()
	request, err := credentialsrequest.NewCredentialsRequest(
		"test-aws-credentials", quay.Namespace,
		"test-aws-sts-credentials", quay.Namespace,
		testSTSRoleARN, stsCloudTokenPath,
		[]string{"test-quay-app"}, []string{testSTSBucket},
	)
	if err != nil {
		t.Fatal(err)
	}
	request.SetOwnerReferences([]metav1.OwnerReference{credentialsRequestOwnerReference(quay)})
	request.SetCreationTimestamp(metav1.NewTime(time.Now().Add(-time.Minute)))
	request.SetGeneration(1)
	return request
}

func testSTSReconciler(t *testing.T, quay *v1.QuayRegistry, objects ...client.Object) *QuayRegistryReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	allObjects := append([]client.Object{quay}, objects...)
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(quay).
		WithObjects(allObjects...).Build()
	return &QuayRegistryReconciler{
		Client:                     cli,
		Log:                        logf.Log.WithName("sts-test"),
		EventRecorder:              record.NewFakeRecorder(20),
		Requeue:                    ctrl.Result{RequeueAfter: 10 * time.Second},
		supportsCredentialsRequest: true,
	}
}

func TestEnsureCredentialsRequestCreatesOwnedRequest(t *testing.T) {
	quay := testQuayRegistry()
	r := testSTSReconciler(t, quay)
	qctx := testSTSContext()

	result, err := r.ensureCredentialsRequest(context.Background(), qctx, quay)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != 10*time.Second {
		t.Fatalf("RequeueAfter = %s, want 10s", result.RequeueAfter)
	}

	request := &unstructured.Unstructured{}
	request.SetGroupVersionKind(credentialsrequest.CredentialsRequestGVK)
	if err := r.Get(context.Background(), types.NamespacedName{
		Name: "test-aws-credentials", Namespace: quay.Namespace,
	}, request); err != nil {
		t.Fatal(err)
	}
	if !credentialsRequestOwnedBy(request, quay) {
		t.Fatal("created CredentialsRequest is not owned by the QuayRegistry")
	}
	roleARN, _, err := unstructured.NestedString(request.Object, "spec", "providerSpec", "stsIAMRoleARN")
	if err != nil || roleARN != testSTSRoleARN {
		t.Fatalf("stsIAMRoleARN = %q, error = %v", roleARN, err)
	}
}

func TestEnsureCredentialsRequestRejectsForeignOwner(t *testing.T) {
	quay := testQuayRegistry()
	request := testCredentialsRequest(t, quay)
	request.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: v1.GroupVersion.String(), Kind: "QuayRegistry",
		Name: "other", UID: types.UID("other-uid"),
	}})
	r := testSTSReconciler(t, quay, request)

	_, err := r.ensureCredentialsRequest(context.Background(), testSTSContext(), quay)
	if err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("error = %v, want ownership conflict", err)
	}
}

func TestEnsureCredentialsRequestUpdatesSpecDrift(t *testing.T) {
	quay := testQuayRegistry()
	request := testCredentialsRequest(t, quay)
	request.SetCreationTimestamp(metav1.NewTime(time.Now().Add(-time.Hour)))
	if err := unstructured.SetNestedField(request.Object, "arn:aws:iam::123456789012:role/old", "spec", "providerSpec", "stsIAMRoleARN"); err != nil {
		t.Fatal(err)
	}
	r := testSTSReconciler(t, quay, request)

	result, err := r.ensureCredentialsRequest(context.Background(), testSTSContext(), quay)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != 10*time.Second {
		t.Fatalf("RequeueAfter = %s, want 10s", result.RequeueAfter)
	}
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(credentialsrequest.CredentialsRequestGVK)
	if err := r.Get(context.Background(), types.NamespacedName{Name: request.GetName(), Namespace: request.GetNamespace()}, updated); err != nil {
		t.Fatal(err)
	}
	roleARN, _, err := unstructured.NestedString(updated.Object, "spec", "providerSpec", "stsIAMRoleARN")
	if err != nil || roleARN != testSTSRoleARN {
		t.Fatalf("updated role ARN = %q, error = %v", roleARN, err)
	}

	// A spec update starts a fresh provisioning timeout even for an old request.
	result, err = r.ensureCredentialsRequest(context.Background(), testSTSContext(), quay)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != 10*time.Second {
		t.Fatalf("second RequeueAfter = %s, want 10s", result.RequeueAfter)
	}
	condition := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeRolloutBlocked)
	if condition == nil || condition.Reason != v1.ConditionReasonCredentialRequestPending {
		t.Fatalf("condition = %#v, want CredentialRequestPending", condition)
	}
}

func TestEnsureCredentialsRequestWaitsForCurrentGeneration(t *testing.T) {
	quay := testQuayRegistry()
	request := testCredentialsRequest(t, quay)
	request.SetGeneration(3)
	if err := unstructured.SetNestedField(request.Object, true, "status", "provisioned"); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(request.Object, int64(2), "status", "lastSyncGeneration"); err != nil {
		t.Fatal(err)
	}
	r := testSTSReconciler(t, quay, request)

	result, err := r.ensureCredentialsRequest(context.Background(), testSTSContext(), quay)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != 10*time.Second {
		t.Fatalf("RequeueAfter = %s, want 10s", result.RequeueAfter)
	}
	condition := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeRolloutBlocked)
	if condition == nil || condition.Reason != v1.ConditionReasonCredentialRequestPending {
		t.Fatalf("condition = %#v, want CredentialRequestPending", condition)
	}
}

func TestEnsureCredentialsRequestTimesOut(t *testing.T) {
	quay := testQuayRegistry()
	request := testCredentialsRequest(t, quay)
	request.SetCreationTimestamp(metav1.NewTime(time.Now().Add(-6 * time.Minute)))
	r := testSTSReconciler(t, quay, request)

	_, err := r.ensureCredentialsRequest(context.Background(), testSTSContext(), quay)
	if err != nil {
		t.Fatal(err)
	}
	condition := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeRolloutBlocked)
	if condition == nil || condition.Reason != v1.ConditionReasonCredentialRequestNotProvisioned {
		t.Fatalf("condition = %#v, want CredentialRequestNotProvisioned", condition)
	}
}

func TestEnsureCredentialsRequestAcceptsCurrentWebIdentitySecret(t *testing.T) {
	quay := testQuayRegistry()
	request := testCredentialsRequest(t, quay)
	if err := unstructured.SetNestedField(request.Object, true, "status", "provisioned"); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(request.Object, int64(1), "status", "lastSyncGeneration"); err != nil {
		t.Fatal(err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-aws-sts-credentials", Namespace: quay.Namespace},
		Data: map[string][]byte{"credentials": []byte(
			"[default]\nrole_arn = " + testSTSRoleARN + "\nweb_identity_token_file = " + stsCloudTokenPath,
		)},
	}
	r := testSTSReconciler(t, quay, request, secret)
	qctx := testSTSContext()

	result, err := r.ensureCredentialsRequest(context.Background(), qctx, quay)
	if err != nil {
		t.Fatal(err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("result = %#v, want zero result", result)
	}
	if !qctx.STSCredentialProvisioned || qctx.STSCredentialSecretName != secret.Name {
		t.Fatalf("STS context not marked provisioned: %#v", qctx)
	}
}

func TestEnsureCredentialsRequestRejectsStaticSecret(t *testing.T) {
	quay := testQuayRegistry()
	request := testCredentialsRequest(t, quay)
	if err := unstructured.SetNestedField(request.Object, true, "status", "provisioned"); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(request.Object, int64(1), "status", "lastSyncGeneration"); err != nil {
		t.Fatal(err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-aws-sts-credentials", Namespace: quay.Namespace},
		Data: map[string][]byte{"credentials": []byte(
			"[default]\naws_access_key_id = AKIA\naws_secret_access_key = secret",
		)},
	}
	r := testSTSReconciler(t, quay, request, secret)

	_, err := r.ensureCredentialsRequest(context.Background(), testSTSContext(), quay)
	if err != nil {
		t.Fatal(err)
	}
	condition := v1.GetCondition(quay.Status.Conditions, v1.ConditionTypeRolloutBlocked)
	if condition == nil || condition.Reason != v1.ConditionReasonCredentialRequestNotProvisioned ||
		!strings.Contains(condition.Message, "static AWS access keys") {
		t.Fatalf("condition = %#v, want static-credential rejection", condition)
	}
}

func TestValidateSTSSharedCredentials(t *testing.T) {
	valid := "[default]\nrole_arn = " + testSTSRoleARN + "\nweb_identity_token_file = " + stsCloudTokenPath
	for _, tt := range []struct {
		name    string
		data    string
		roleARN string
		token   string
		wantErr string
	}{
		{name: "valid", data: valid, roleARN: testSTSRoleARN, token: stsCloudTokenPath},
		{name: "empty", wantErr: "empty"},
		{name: "static keys", data: "[default]\naws_access_key_id = AKIA\naws_secret_access_key = secret", roleARN: testSTSRoleARN, token: stsCloudTokenPath, wantErr: "static"},
		{name: "wrong role", data: valid, roleARN: "arn:aws:iam::123:role/other", token: stsCloudTokenPath, wantErr: "role_arn"},
		{name: "wrong token path", data: valid, roleARN: testSTSRoleARN, token: "/wrong", wantErr: "web_identity_token_file"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSTSSharedCredentials([]byte(tt.data), tt.roleARN, tt.token)
			if tt.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCleanupCredentialsRequestHonorsOwnership(t *testing.T) {
	for _, tt := range []struct {
		name      string
		owned     bool
		wantFound bool
	}{
		{name: "owned request is deleted", owned: true},
		{name: "foreign request is preserved", wantFound: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			quay := testQuayRegistry()
			request := testCredentialsRequest(t, quay)
			if !tt.owned {
				request.SetOwnerReferences(nil)
			}
			r := testSTSReconciler(t, quay, request)

			if err := r.cleanupCredentialsRequest(context.Background(), quay); err != nil {
				t.Fatal(err)
			}
			fetched := &unstructured.Unstructured{}
			fetched.SetGroupVersionKind(credentialsrequest.CredentialsRequestGVK)
			err := r.Get(context.Background(), types.NamespacedName{Name: request.GetName(), Namespace: request.GetNamespace()}, fetched)
			if tt.wantFound && err != nil {
				t.Fatalf("foreign request should remain: %v", err)
			}
			if !tt.wantFound && !apierrors.IsNotFound(err) {
				t.Fatalf("owned request should be deleted, get error = %v", err)
			}
		})
	}
}
