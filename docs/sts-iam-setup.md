# AWS STS authentication for unmanaged S3 storage

The Quay Operator can use OpenShift's Cloud Credential Operator (CCO) to give Quay workloads short-lived AWS credentials. This avoids storing IAM user access keys in the Quay configuration.

This workflow requires:

- OpenShift 4.14 or later on AWS;
- a cluster installed for AWS Security Token Service (STS), with CCO in `Manual` mode and a configured service-account issuer;
- an IAM role trusted by the OpenShift cluster's OIDC provider;
- `objectstorage` set to `managed: false`;
- a Quay `S3Storage` configuration without `s3_access_key` or `s3_secret_key`.

Managed object storage such as NooBaa handles its own credentials and does not use this flow. The legacy Quay `STSS3Storage` driver is also not used; CCO's web-identity credentials file works with the standard `S3Storage` driver.

## 1. Create the IAM policy

Replace `QUAY_BUCKET` and `QUAY_ROLE_NAME` before running these commands. The policy is limited to the configured bucket and the S3 operations Quay requires.

```bash
export QUAY_BUCKET=example-quay-bucket
export QUAY_ROLE_NAME=example-quay-sts
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export AWS_PARTITION="$(aws sts get-caller-identity --query Arn --output text | cut -d: -f2)"

cat >quay-s3-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket",
        "s3:GetBucketLocation",
        "s3:ListBucketMultipartUploads"
      ],
      "Resource": "arn:${AWS_PARTITION}:s3:::${QUAY_BUCKET}"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:AbortMultipartUpload",
        "s3:ListMultipartUploadParts"
      ],
      "Resource": "arn:${AWS_PARTITION}:s3:::${QUAY_BUCKET}/*"
    }
  ]
}
EOF

aws iam create-policy \
  --policy-name "${QUAY_ROLE_NAME}" \
  --policy-document file://quay-s3-policy.json
```

The detected `AWS_PARTITION` also supports partitions such as AWS GovCloud (`aws-us-gov`).

## 2. Create the role trust policy

The role must trust the service account used by the Quay application and mirror workloads. Its name is `<QuayRegistry name>-quay-app`.

```bash
export QUAY_NAMESPACE=example-quay
export QUAY_REGISTRY_NAME=example
export QUAY_SERVICE_ACCOUNT="${QUAY_REGISTRY_NAME}-quay-app"
export OIDC_ISSUER="$(oc get authentication cluster -o jsonpath='{.spec.serviceAccountIssuer}')"
export OIDC_PROVIDER="${OIDC_ISSUER#https://}"

cat >quay-trust-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "${OIDC_PROVIDER}:aud": "openshift",
          "${OIDC_PROVIDER}:sub": "system:serviceaccount:${QUAY_NAMESPACE}:${QUAY_SERVICE_ACCOUNT}"
        }
      }
    }
  ]
}
EOF

aws iam create-role \
  --role-name "${QUAY_ROLE_NAME}" \
  --assume-role-policy-document file://quay-trust-policy.json

aws iam attach-role-policy \
  --role-name "${QUAY_ROLE_NAME}" \
  --policy-arn "arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:policy/${QUAY_ROLE_NAME}"

export QUAY_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${QUAY_ROLE_NAME}"
```

If one operator installation manages multiple Quay registries, the role's trust policy must allow each registry service-account subject, or each operator installation must be configured with a role appropriate for its managed registries.

## 3. Install or update the Operator with `ROLEARN`

For a CLI installation, set `ROLEARN` in the OLM Subscription. Manual install-plan approval is recommended so permission changes can be reviewed during upgrades.

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: quay-operator
  namespace: openshift-operators
spec:
  channel: stable-3.19
  installPlanApproval: Manual
  name: quay-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
  config:
    env:
      - name: ROLEARN
        value: arn:aws:iam::123456789012:role/example-quay-sts
```

The OpenShift console displays the AWS role field for Operator versions declaring token-authentication support.

## 4. Configure credential-free S3 storage

Create a config bundle containing `S3Storage` without static access keys:

```yaml
DISTRIBUTED_STORAGE_CONFIG:
  default:
    - S3Storage
    - host: s3.amazonaws.com
      port: 443
      s3_bucket: example-quay-bucket
      storage_path: /datastorage/registry
DISTRIBUTED_STORAGE_DEFAULT_LOCATIONS:
  - default
DISTRIBUTED_STORAGE_PREFERENCE:
  - default
```

```bash
oc create secret generic example-config \
  -n example-quay \
  --from-file=config.yaml
```

Reference that Secret and mark object storage unmanaged:

```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: example
  namespace: example-quay
spec:
  configBundleSecret: example-config
  components:
    - kind: objectstorage
      managed: false
```

When reconciliation succeeds, the Operator creates `example-aws-credentials`. CCO provisions `example-aws-sts-credentials`, and the Operator mounts it into the Quay application and mirror pods.

## 5. Verify the integration

Inspect status without printing credential or token contents:

```bash
oc get quayregistry example -n example-quay \
  -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'

oc get credentialsrequest example-aws-credentials -n example-quay \
  -o jsonpath='{.status.provisioned}{"\t"}{.status.lastSyncGeneration}{"\n"}'

oc get deployment example-quay-app -n example-quay \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="AWS_SHARED_CREDENTIALS_FILE")].value}{"\n"}'
```

The environment value should be `/aws-sts/credentials`. Complete validation by pushing and pulling an image and, when enabled, running a repository mirror.

Do not print or copy the projected service-account token into logs or support cases.

## Conditions and troubleshooting

| Reason | Meaning | Action |
|---|---|---|
| `CredentialRequestPending` | CCO has not yet provisioned the current request generation | Wait for CCO reconciliation and inspect the CredentialsRequest status |
| `CredentialRequestNotProvisioned` | Provisioning timed out or the generated credentials are not valid web-identity credentials | Verify AWS platform, CCO Manual mode, OIDC issuer, role ARN, and CCO logs |
| `ConflictingCredentials` | The S3 config contains `s3_access_key` or `s3_secret_key` | Remove static keys from every `S3Storage` entry |
| `ConfigInvalid` with an STS message | The cluster, backend, or CCO mode cannot support the requested flow | Correct the reported prerequisite or remove `ROLEARN` |

Useful checks:

```bash
oc get infrastructure cluster -o jsonpath='{.status.platformStatus.type}{"\n"}'
oc get cloudcredential cluster -o jsonpath='{.spec.credentialsMode}{"\n"}'
oc get authentication cluster -o jsonpath='{.spec.serviceAccountIssuer}{"\n"}'
oc get credentialsrequest -n example-quay
```

Removing `ROLEARN`, changing to managed object storage, or changing away from `S3Storage` causes the Operator to delete its CredentialsRequest and remove the STS mounts on the next successful reconciliation.

## Standalone RHEL-based Quay

CCO and the Quay Operator are not present in a standalone RHEL deployment. The administrator must provide an OIDC token, keep it refreshed, and mount both it and an AWS shared credentials file into the Quay container.

Example credentials file:

```ini
[default]
role_arn = arn:aws:iam::123456789012:role/example-quay-sts
web_identity_token_file = /run/secrets/quay/aws-web-identity-token
```

Mount the files read-only and set:

```text
AWS_SHARED_CREDENTIALS_FILE=/run/secrets/quay/aws-credentials
```

Continue to use `S3Storage` without static keys in `config.yaml`. The external identity system is responsible for issuing and rotating the token at `web_identity_token_file`; Quay does not create that token or configure the AWS OIDC provider. Test the token with AWS `AssumeRoleWithWebIdentity` before starting Quay.

## Further reading

- [OpenShift: enabling CCO-based AWS STS for Operators](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/operators/developing-operators#osdk-cco-aws-sts)
- [Cloud Credential Operator modes](https://github.com/openshift/cloud-credential-operator/blob/release-4.14/README.md#4-short-lived-tokens)
- [AWS AssumeRoleWithWebIdentity](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html)
