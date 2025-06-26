import os

from ruamel.yaml import YAML

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


quay_operator_metadata_annotations = load_manifest(
    os.getenv("TARGET_METADATA_ANNOTATIONS_FILE")
)

# Set common variables
name = "quay-operator"
channel = "stable-3.14"

# Start updating yaml...
quay_operator_metadata_annotations["annotations"][
    "operators.operatorframework.io.bundle.package.v1"
] = name
quay_operator_metadata_annotations["annotations"][
    "operators.operatorframework.io.bundle.channel.default.v1"
] = channel
quay_operator_metadata_annotations["annotations"][
    "operators.operatorframework.io.bundle.channels.v1"
] = channel

dump_manifest(
    os.getenv("TARGET_METADATA_ANNOTATIONS_FILE"), quay_operator_metadata_annotations
)
