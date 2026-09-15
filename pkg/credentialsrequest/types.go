package credentialsrequest

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// These wire types intentionally mirror cloudcredential.openshift.io/v1 in
// OpenShift 4.14+, without importing CCO's dependency-heavy internal API.
// Source: github.com/openshift/cloud-credential-operator/pkg/apis/cloudcredential/v1
var CredentialsRequestGVK = schema.GroupVersionKind{
	Group:   "cloudcredential.openshift.io",
	Version: "v1",
	Kind:    "CredentialsRequest",
}

type AWSProviderSpec struct {
	APIVersion       string           `json:"apiVersion"`
	Kind             string           `json:"kind"`
	StatementEntries []StatementEntry `json:"statementEntries"`
	STSIAMRoleARN    string           `json:"stsIAMRoleARN,omitempty"`
}

type StatementEntry struct {
	Effect   string   `json:"effect"`
	Action   []string `json:"action"`
	Resource string   `json:"resource"`
}

type SecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type CredentialsRequestSpec struct {
	SecretRef           SecretRef       `json:"secretRef"`
	ProviderSpec        json.RawMessage `json:"providerSpec"`
	ServiceAccountNames []string        `json:"serviceAccountNames"`
	CloudTokenPath      string          `json:"cloudTokenPath"`
}

func NewAWSProviderSpec(roleARN string, statements []StatementEntry) (json.RawMessage, error) {
	spec := AWSProviderSpec{
		APIVersion:       "cloudcredential.openshift.io/v1",
		Kind:             "AWSProviderSpec",
		StatementEntries: statements,
		STSIAMRoleARN:    roleARN,
	}
	return json.Marshal(spec)
}

func NewCredentialsRequest(
	name, namespace string,
	secretRefName, secretRefNamespace string,
	roleARN, cloudTokenPath string,
	serviceAccountNames, buckets []string,
) (*unstructured.Unstructured, error) {
	if len(buckets) == 0 {
		return nil, fmt.Errorf("at least one S3 bucket is required")
	}

	partition, err := awsPartitionFromRoleARN(roleARN)
	if err != nil {
		return nil, err
	}

	statements := make([]StatementEntry, 0, len(buckets)*2)
	for _, bucket := range buckets {
		bucket = strings.TrimSpace(bucket)
		if bucket == "" {
			return nil, fmt.Errorf("S3 bucket must not be empty")
		}
		if strings.ContainsAny(bucket, "*?") {
			return nil, fmt.Errorf("S3 bucket %q contains wildcard characters", bucket)
		}
		bucketARN := fmt.Sprintf("arn:%s:s3:::%s", partition, bucket)
		statements = append(statements,
			StatementEntry{
				Effect: "Allow",
				Action: []string{
					"s3:ListBucket",
					"s3:GetBucketLocation",
					"s3:ListBucketMultipartUploads",
				},
				Resource: bucketARN,
			},
			StatementEntry{
				Effect: "Allow",
				Action: []string{
					"s3:GetObject",
					"s3:PutObject",
					"s3:DeleteObject",
					"s3:AbortMultipartUpload",
					"s3:ListMultipartUploadParts",
				},
				Resource: bucketARN + "/*",
			},
		)
	}

	providerSpec, err := NewAWSProviderSpec(roleARN, statements)
	if err != nil {
		return nil, err
	}

	spec := CredentialsRequestSpec{
		SecretRef: SecretRef{
			Name:      secretRefName,
			Namespace: secretRefNamespace,
		},
		ProviderSpec:        providerSpec,
		ServiceAccountNames: serviceAccountNames,
		CloudTokenPath:      cloudTokenPath,
	}

	specJSON, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var specMap map[string]interface{}
	if err := json.Unmarshal(specJSON, &specMap); err != nil {
		return nil, err
	}

	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": CredentialsRequestGVK.Group + "/" + CredentialsRequestGVK.Version,
			"kind":       CredentialsRequestGVK.Kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": specMap,
		},
	}

	return cr, nil
}

func awsPartitionFromRoleARN(roleARN string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(roleARN), ":", 6)
	if len(parts) != 6 || parts[0] != "arn" || parts[1] == "" || parts[2] != "iam" ||
		parts[3] != "" || parts[4] == "" || !strings.HasPrefix(parts[5], "role/") || len(parts[5]) == len("role/") {
		return "", fmt.Errorf("ROLEARN must be a valid AWS IAM role ARN")
	}
	return parts[1], nil
}
