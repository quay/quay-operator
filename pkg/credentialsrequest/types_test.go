package credentialsrequest

import (
	"encoding/json"
	"testing"
)

func TestNewCredentialsRequest(t *testing.T) {
	cr, err := NewCredentialsRequest(
		"test-aws-credentials", "test-namespace",
		"test-aws-sts-credentials", "test-namespace",
		"arn:aws:iam::123456789012:role/quay-role",
		"/var/run/secrets/openshift/serviceaccount/token",
		[]string{"test-quay-app"}, []string{"quay-bucket"},
	)
	if err != nil {
		t.Fatalf("NewCredentialsRequest() error = %v", err)
	}

	t.Run("correct GVK", func(t *testing.T) {
		gvk := cr.GroupVersionKind()
		if gvk.Group != "cloudcredential.openshift.io" {
			t.Errorf("Group = %q, want %q", gvk.Group, "cloudcredential.openshift.io")
		}
		if gvk.Version != "v1" {
			t.Errorf("Version = %q, want %q", gvk.Version, "v1")
		}
		if gvk.Kind != "CredentialsRequest" {
			t.Errorf("Kind = %q, want %q", gvk.Kind, "CredentialsRequest")
		}
	})

	t.Run("correct metadata", func(t *testing.T) {
		if cr.GetName() != "test-aws-credentials" {
			t.Errorf("Name = %q, want %q", cr.GetName(), "test-aws-credentials")
		}
		if cr.GetNamespace() != "test-namespace" {
			t.Errorf("Namespace = %q, want %q", cr.GetNamespace(), "test-namespace")
		}
	})

	t.Run("correct spec fields", func(t *testing.T) {
		spec, ok := cr.Object["spec"].(map[string]interface{})
		if !ok {
			t.Fatal("spec is not a map")
		}

		secretRef, ok := spec["secretRef"].(map[string]interface{})
		if !ok {
			t.Fatal("secretRef is not a map")
		}
		if secretRef["name"] != "test-aws-sts-credentials" {
			t.Errorf("secretRef.name = %v, want %q", secretRef["name"], "test-aws-sts-credentials")
		}
		if secretRef["namespace"] != "test-namespace" {
			t.Errorf("secretRef.namespace = %v, want %q", secretRef["namespace"], "test-namespace")
		}

		if spec["cloudTokenPath"] != "/var/run/secrets/openshift/serviceaccount/token" {
			t.Errorf("cloudTokenPath = %v, want %q", spec["cloudTokenPath"], "/var/run/secrets/openshift/serviceaccount/token")
		}

		saNames, ok := spec["serviceAccountNames"].([]interface{})
		if !ok {
			t.Fatal("serviceAccountNames is not a slice")
		}
		if len(saNames) != 1 || saNames[0] != "test-quay-app" {
			t.Errorf("serviceAccountNames = %v, want [test-quay-app]", saNames)
		}
	})

	t.Run("providerSpec contains role ARN", func(t *testing.T) {
		spec := cr.Object["spec"].(map[string]interface{})
		providerSpec := spec["providerSpec"]
		providerJSON, err := json.Marshal(providerSpec)
		if err != nil {
			t.Fatalf("failed to marshal providerSpec: %v", err)
		}

		var ps AWSProviderSpec
		if err := json.Unmarshal(providerJSON, &ps); err != nil {
			t.Fatalf("failed to unmarshal providerSpec: %v", err)
		}

		if ps.STSIAMRoleARN != "arn:aws:iam::123456789012:role/quay-role" {
			t.Errorf("stsIAMRoleARN = %q, want %q", ps.STSIAMRoleARN, "arn:aws:iam::123456789012:role/quay-role")
		}
		if ps.Kind != "AWSProviderSpec" {
			t.Errorf("Kind = %q, want %q", ps.Kind, "AWSProviderSpec")
		}
		if len(ps.StatementEntries) != 2 {
			t.Fatalf("len(StatementEntries) = %d, want 2", len(ps.StatementEntries))
		}
		if ps.StatementEntries[0].Effect != "Allow" {
			t.Errorf("Effect = %q, want %q", ps.StatementEntries[0].Effect, "Allow")
		}
		if ps.StatementEntries[0].Resource != "arn:aws:s3:::quay-bucket" {
			t.Errorf("bucket Resource = %q", ps.StatementEntries[0].Resource)
		}
		if ps.StatementEntries[1].Resource != "arn:aws:s3:::quay-bucket/*" {
			t.Errorf("object Resource = %q", ps.StatementEntries[1].Resource)
		}
	})
}

func TestNewCredentialsRequestRequiresBucket(t *testing.T) {
	_, err := NewCredentialsRequest(
		"test", "test-ns", "secret", "test-ns",
		"arn:aws:iam::123456789012:role/quay-role",
		"/var/run/secrets/openshift/serviceaccount/token",
		[]string{"test-quay-app"}, nil,
	)
	if err == nil {
		t.Fatal("expected an error when no S3 bucket is supplied")
	}
}

func TestNewCredentialsRequestRejectsInvalidScope(t *testing.T) {
	for _, tt := range []struct {
		name    string
		roleARN string
		bucket  string
	}{
		{name: "malformed role ARN", roleARN: "not-an-arn", bucket: "quay-bucket"},
		{name: "non-IAM ARN", roleARN: "arn:aws:s3:::bucket", bucket: "quay-bucket"},
		{name: "bucket wildcard", roleARN: "arn:aws:iam::123456789012:role/quay-role", bucket: "quay-*"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCredentialsRequest(
				"test", "test-ns", "secret", "test-ns",
				tt.roleARN, "/var/run/token", []string{"test-quay-app"}, []string{tt.bucket},
			)
			if err == nil {
				t.Fatal("expected invalid scope to be rejected")
			}
		})
	}
}

func TestNewCredentialsRequestUsesRolePartition(t *testing.T) {
	cr, err := NewCredentialsRequest(
		"test", "test-ns", "secret", "test-ns",
		"arn:aws-us-gov:iam::123456789012:role/quay-role",
		"/var/run/secrets/openshift/serviceaccount/token",
		[]string{"test-quay-app"}, []string{"quay-bucket"},
	)
	if err != nil {
		t.Fatal(err)
	}
	spec := cr.Object["spec"].(map[string]interface{})
	providerJSON, err := json.Marshal(spec["providerSpec"])
	if err != nil {
		t.Fatal(err)
	}
	var provider AWSProviderSpec
	if err := json.Unmarshal(providerJSON, &provider); err != nil {
		t.Fatal(err)
	}
	if got := provider.StatementEntries[0].Resource; got != "arn:aws-us-gov:s3:::quay-bucket" {
		t.Fatalf("Resource = %q, want GovCloud partition", got)
	}
}
