#!/usr/bin/env bash
set -x

# Make sure we have all necessary environment variables
source render_vars # this is from cpaas (version numbers etc)


export MANIFESTS_DIR="../bundle/manifests"
export METADATA_DIR="../bundle/metadata"

export X_VERSION=${CI_X_VERSION}
export Y_VERSION=${CI_Y_VERSION}
export Z_VERSION=${CI_Z_VERSION}
export CI_VERSION=v${CI_VERSION}
export CI_VERSION_SANITIZED=${CI_VERSION}
export UPSTREAM_VERSION_SANITIZED=${CI_UPSTREAM_VERSION_SANITIZED}
export SCRIPT_CLONE_URL=${CI_UPSTREAM_URL}
export SCRIPT_CLONE_SHA=${CI_UPSTREAM_COMMIT}

# script_target_dir="scripts"

# Get our tools and unbound manifests from upstream.
# git config --global core.sshCommand 'ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no'
# git clone $SCRIPT_CLONE_URL $script_target_dir
# if [[ $? -ne 0 ]]; then
#    echo "ERROR: Could not clone upstream release repo."
#    exit 2
# fi
#
# cd $script_target_dir
# git checkout $SCRIPT_CLONE_SHA
# cd ..

# Grab the directories from the script dir so that they can be copied into the final image
# rm -rf ./${MANIFESTS_DIR} ./${METADATA_DIR}
# mkdir -p ./${MANIFESTS_DIR} ./${METADATA_DIR} # Ensure that directories exist in case they are nested
# mv $script_target_dir/bundle/manifests/* ./${MANIFESTS_DIR}
# mv $script_target_dir/bundle/metadata/* ./${METADATA_DIR}

# Downstream images currently
registry="registry-proxy.engineering.redhat.com/rh-osbs"

# Eventually...
# registry="quay.io/redhat-user-workloads/quay-eng-tenant"

operator_image="${registry}/quay-quay-operator-rhel8:${CI_CONTAINER_VERSION}"
quay_image="${registry}/quay-quay-rhel8:${CI_CONTAINER_VERSION}"
clair_image="${registry}/quay-clair-rhel8:${CI_CONTAINER_VERSION}"
quay_builder_image="${registry}/quay-quay-builder-rhel8:${CI_CONTAINER_VERSION}"
quay_builder_qemu_image="${registry}/quay-quay-builder-qemu-rhcos-rhel8:${CI_CONTAINER_VERSION}"

# https://catalog.redhat.com/software/containers/rhel8/postgresql-13/5ffdbdef73a65398111b8362
# release-tool:follow(registry.redhat.io/rhel8/postgresql-13)
postgres_image="registry.redhat.io/rhel8/postgresql-13:1-216.1749685127"

# https://catalog.redhat.com/software/containers/rhel8/postgresql-10/5ba0ae0ddd19c70b45cbf4cd
# release-tool:follow(registry.redhat.io/rhel8/postgresql-10)
postgres_image_previous="registry.redhat.io/rhel8/postgresql-10:1-245.1717586538"

# https://catalog.redhat.com/software/containers/rhel8/postgresql-15/63d29a05fd1c4f5552a305b3
# release-tool:follow(registry.redhat.io/rhel8/postgresql-15)
clair_postgres_image="registry.redhat.io/rhel8/postgresql-15:1-93.1749685130"

# https://catalog.redhat.com/software/containers/rhel8/postgresql-13/5ffdbdef73a65398111b8362
# release-tool:follow(registry.redhat.io/rhel8/postgresql-13)
clair_postgres_image_previous="registry.redhat.io/rhel8/postgresql-13:1-216.1749685127"

# https://catalog.redhat.com/software/containers/rhel8/redis-6/6065b06cdfe097aa13042b50
# release-tool:follow(registry.redhat.io/rhel8/redis-6)
redis_image="registry.redhat.io/rhel8/redis-6:1-215.1749685126"

export OPERATOR_IMAGE=${operator_image}
export QUAY_IMAGE=${quay_image}
export CLAIR_IMAGE=${clair_image}
export QUAY_BUILDER_IMAGE=${quay_builder_image}
export QUAY_BUILDER_QEMU_IMAGE=${quay_builder_qemu_image}
export POSTGRES_IMAGE=${postgres_image}
export POSTGRES_IMAGE_PREVIOUS=${postgres_image_previous}
export CLAIR_POSTGRES_IMAGE=${clair_postgres_image}
export CLAIR_POSTGRES_IMAGE_PREVIOUS=${clair_postgres_image_previous}
export REDIS_IMAGE=${redis_image}

# Manifest file locations
csv_name="quay-operator.clusterserviceversion.yaml"
csv_file="${MANIFESTS_DIR}/${csv_name}"
if [ ! -f $csv_file ]; then
   echo "CSV file not found, the version or name might have changed on us!"
   exit 5
fi
metadata_annotatations_name="annotations.yaml"
metadata_annotatations_file="${METADATA_DIR}/${metadata_annotatations_name}"

export TARGET_CSV_FILE="${csv_file}"
export TARGET_METADATA_ANNOTATIONS_FILE="${metadata_annotatations_file}"
export EPOC_TIMESTAMP=$(date +%s)

# time for some direct modifications to the csv
pip3 install --upgrade pip && pip3 install -r requirements.txt
python3 update_csv.py
python3 update_metadata.py
