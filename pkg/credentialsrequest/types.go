package credentialsrequest

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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
	serviceAccountNames []string,
) (*unstructured.Unstructured, error) {
	providerSpec, err := NewAWSProviderSpec(roleARN, []StatementEntry{
		{
			Effect: "Allow",
			Action: []string{
				"s3:GetObject",
				"s3:PutObject",
				"s3:DeleteObject",
				"s3:ListBucket",
				"s3:GetBucketLocation",
				"s3:ListBucketMultipartUploads",
				"s3:AbortMultipartUpload",
				"s3:ListMultipartUploadParts",
			},
			Resource: "arn:aws:s3:*:*:*",
		},
	})
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
