# Upgrades

The Quay Operator follows a _synchronized versioning_ scheme, which means that each version of the Operator is tied to the version of Quay and its components which it manages. There is no field on the `QuayRegistry` custom resource which sets the version of Quay to deploy; the Operator only knows how to deploy a single version of all components. This scheme was chosen to ensure that all components work well together and to reduce complexity of the Operator needing to know how to manage the lifecycles of many different versions of Quay on Kubernetes.

### Operator Lifecycle Manager

The Quay Operator should be installed and upgraded using the [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager) (OLM). This powerful and complex piece of software takes care of the full lifecycle of Operators, including installation, configuration, automatic upgrades, UX enhancements, etc. When creating a `Subscription` with the default `approvalStrategy: Automatic`, OLM will automatically upgrade the Quay Operator whenever a new version becomes available. **NOTE**: When the Quay Operator upgrades automatically, it will perform upgrades on any `QuayRegistries` it finds, bumping them to match the Operator's own version. If you want control over upgrades to your Quay registry, set `approvalStrategy: Manual`.

### From QuayRegistry

When the Quay Operator starts up, it immediately looks for any `QuayRegistries` it can find in the namespace(s) it is configured to watch. When it finds one, the following logic is used:

If `status.currentVersion` is unset, reconcile as normal.
If `status.currentVersion` equals the Operator version, reconcile as normal.
If `status.currentVersion` does not equal the Operator version, check if it can be upgraded. If it can, perform upgrade tasks and set the `status.currentVersion` to the Operator's version once complete. If it cannot be upgraded, return an error and leave the `QuayRegistry` and its deployed Kubernetes objects alone.

### From QuayEcosystem

Upgrades are supported from previous versions of the Operator which used the `QuayEcosystem` API. To ensure that migrations do not happen unexpectedly, a special label needs to be applied to the `QuayEcosystem` for it to be migrated. A new `QuayRegistry` will be created for the Operator to manage, but the old `QuayEcosystem` will remain until manually deleted to ensure that you can roll back and still access Quay in case anything goes wrong. To migrate an existing `QuayEcosystem` to a new `QuayRegistry`, follow these steps:

1. Ensure that the existing managed Postgres database has a password set for the `postgres` root admin user (not set by default). This allows remote access to the database for migration. The Operator looks for this password in the `Secret` referenced by `spec.quay.database.credentialsSecretKey` under the `database-root-password` key.

 To set/change the password, use either the OpenShift console or `kubectl` to [open an SSH terminal connection](https://kubernetes.io/docs/tasks/debug-application-cluster/get-shell-running-container/) to the Postgres pod:
```sh
$ kubectl exec -n <namespace> --stdin --tty deployment/<quayecosystem-name>-quay-postgresql -- /bin/bash
```

Execute the following command and follow instructions to change the password:
```sh
$ psql
psql (10.12)
Type "help" for help.

postgres=# \password
```

2. Add `"quay-operator/migrate": "true"` to the `metadata.labels` of the `QuayEcosystem`.

3. Wait for a `QuayRegistry` to be created with the same `metadata.name` as your `QuayEcosystem`.

4. Once the `status.registryEndpoint` of the new `QuayRegistry` is set, access Quay and confirm all data and settings were migrated successfully.

5. When you are confident everything worked correctly, you may delete the `QuayEcosystem` and Kubernetes garbage collection will clean up all old resources.

#### Reverting QuayEcosystem Upgrade

If something goes wrong during the automatic upgrade from `QuayEcosystem` to `QuayRegistry`, follow these steps to revert back to using the `QuayEcosystem`:

1. Delete the `QuayRegistry` using either the UI or `kubectl`:
```sh
$ kubectl delete -n <namespace> quayregistry <quayecosystem-name>
```

2. If external access was provided using a `Route`, change the `Route` to point back to the original `Service` using the UI or `kubectl`.
