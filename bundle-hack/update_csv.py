import os
from datetime import datetime

from ruamel.yaml import YAML
from ruamel.yaml.scalarstring import PreservedScalarString as pss

yaml = YAML()


def load_manifest(pathn):
    if not pathn.endswith(".yaml"):
        return None
    try:
        with open(pathn, "r") as f:
            return yaml.load(f)
    except FileNotFoundError:
        print("File can not found")
        exit(2)


def dump_manifest(pathn, manifest):
    with open(pathn, "w") as f:
        yaml.dump(manifest, f)
    return


quay_operator_csv = load_manifest(os.getenv("TARGET_CSV_FILE"))

# Set common variables
major_version = os.getenv("X_VERSION")
minor_version = int(os.getenv("Y_VERSION"))
patch_version = int(os.getenv("Z_VERSION"))
current_version = os.getenv("CI_VERSION_SANITIZED")
current_version_full = os.getenv("CI_VERSION")
old_version = f"{major_version}.{minor_version}.{patch_version - 1}"
operator_name = "quay-operator"
name = f"{operator_name}.v{current_version}"
image = os.getenv("OPERATOR_IMAGE") + current_version_full
timestamp = int(os.getenv("EPOC_TIMESTAMP"))
datetime_time = datetime.fromtimestamp(timestamp)

# Map upstream images to downstream images
image_map = {
    "QUAY_DEFAULT_BRANDING": "redhat",
    "RELATED_IMAGE_COMPONENT_QUAY": os.getenv("QUAY_IMAGE") + current_version_full,
    "RELATED_IMAGE_COMPONENT_CLAIR": os.getenv("CLAIR_IMAGE") + current_version_full,
    "RELATED_IMAGE_COMPONENT_BUILDER": os.getenv("QUAY_BUILDER_IMAGE")
    + current_version_full,
    "RELATED_IMAGE_COMPONENT_BUILDER_QEMU": os.getenv("QUAY_BUILDER_QEMU_IMAGE")
    + current_version_full,
    "RELATED_IMAGE_COMPONENT_POSTGRES": os.getenv("POSTGRES_IMAGE"),
    "RELATED_IMAGE_COMPONENT_POSTGRES_PREVIOUS": os.getenv("POSTGRES_IMAGE_PREVIOUS"),
    "RELATED_IMAGE_COMPONENT_CLAIRPOSTGRES": os.getenv("CLAIR_POSTGRES_IMAGE"),
    "RELATED_IMAGE_COMPONENT_CLAIRPOSTGRES_PREVIOUS": os.getenv("CLAIR_POSTGRES_IMAGE_PREVIOUS"),
    "RELATED_IMAGE_COMPONENT_REDIS": os.getenv("REDIS_IMAGE"),
}

verbose_description = pss(
    """\
    The Red Hat Quay Operator deploys and manages a production-ready
    [Red Hat Quay](https://www.openshift.com/products/quay) private container registry.
    This operator provides an opinionated installation and configuration of Red Hat Quay.
    All components required, including Clair, database, and storage, are provided in an
    operator-managed fashion. Each component may optionally be self-managed.

    ## Operator Features

    * Automated installation of Red Hat Quay
    * Provisions instance of Redis
    * Provisions PostgreSQL to support both Quay and Clair
    * Installation of Clair for container scanning and integration with Quay
    * Provisions and configures ODF for supported registry object storage
    * Enables and configures Quay's registry mirroring feature

    ## Prerequisites

    By default, the Red Hat Quay Operator expects ODF to be installed on the cluster to
    provide the _ObjectBucketClaim_ API for object storage. For production deployment of
    Red Hat OpenShift Data Foundation, please refer to the
    [official documentation](https://access.redhat.com/documentation/en-us/red_hat_openshift_container_storage/).

    ## Simplified Deployment

    The following example provisions a fully operator-managed deployment of Red Hat Quay,
    including all services necessary for production:

    ```
    apiVersion: quay.redhat.com/v1
    kind: QuayRegistry
    metadata:
      name: my-registry
    ```

    ## Documentation

    See the
    [official documentation](https://access.redhat.com/documentation/en-us/red_hat_quay/3/html/deploying_the_red_hat_quay_operator_on_openshift_container_platform/index)
    for more complex deployment scenarios and information.
"""
)

# Start updating yaml...
quay_operator_csv["metadata"]["name"] = name

# Commenting out this at the moment because it forces a rebuild everytime. 
# quay_operator_csv["metadata"]["annotations"]["createdAt"] = datetime_time.strftime(
#     "%d %b %Y, %H:%M"
# ) 
quay_operator_csv["spec"]["maintainers"] = [
    {"email": "support@redhat.com", "name": "Red Hat"}
]
# quay_operator_csv["spec"]["icon"][0]["base64data"] = os.getenv("BASE64_ICON")
quay_operator_csv["spec"]["icon"][0]["mediatype"] = "image/svg+xml"
quay_operator_csv["spec"]["description"] = verbose_description
quay_operator_csv["spec"]["displayName"] = "Red Hat Quay"
quay_operator_csv["spec"]["version"] = current_version

# If this is a Y-stream release, remove replaces
if patch_version == 0:
    if "replaces" in quay_operator_csv["spec"]:
        quay_operator_csv["spec"].pop("replaces")
else:
    quay_operator_csv["spec"]["replaces"] = f"{operator_name}.v{old_version}" 

annotations = quay_operator_csv["metadata"]["annotations"]
annotations["olm.skipRange"] = f">={major_version}.{minor_version - 3}.x <{current_version}"
annotations["containerImage"] = image
annotations["quay-version"] = current_version_full
annotations["description"] = annotations["description"].replace("Quay", "Red Hat")
annotations[
    "operators.openshift.io/valid-subscription"
] = '["OpenShift Platform Plus", "Red Hat Quay"]'

# Drill down into deployments
deployments = quay_operator_csv["spec"]["install"]["spec"]["deployments"]
deployment = {}
for obj in deployments:
    if operator_name in obj["name"]:
        deployment = obj
deployment["name"] = name

# Drill down into containers and set container image
containers = deployment["spec"]["template"]["spec"]["containers"]
container = {}
for obj in containers:
    if operator_name in obj["name"]:
        container = obj

container["image"] = image

# Drill down into env and update container images
for var in container["env"]:
    if var["name"] in image_map:
        var["value"] = image_map[var["name"]]


dump_manifest(os.getenv("TARGET_CSV_FILE"), quay_operator_csv)
