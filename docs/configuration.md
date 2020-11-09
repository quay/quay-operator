# Configuration

Quay is a powerful container registry platform with many features and components, and is highly configurable. The Quay Operator attempts to mitigate the potential headaches of configuration in a few different ways. 

## Quay Config File

All of Quay's config is contained in a single file called `config.yaml`, which is defined in this [schema](https://github.com/quay/quay/blob/master/util/config/schema.py). The Quay application requires this file to be mounted into the container at a specific location at runtime. On Kubernetes, this is accomplished by creating a `Secret` with the `config.yaml` file contents and referencing it as a `volumeMount` in the Quay `Deployment`.

The Quay Operator uses the `spec.configBundleSecret` field of the `QuayRegistry` API to provide a base config to the Quay application that will be deployed. This `Secret` is merged with other config values to produce a final config bundle which is actually mounted into the Quay containers. If you are interested in seeing the full config being used by Quay, simply inspect the Quay `Deployment` and find the `Secret` referenced by the `volumeMount`:

```sh
$ kubectl get deployments -n <namespace> <quayregistry-name>-quay-app -o jsonpath="{.spec.template.spec.volumes[0].secret.secretName}"
<quayregistry-name>-quay-config-secret-22dbk9btcg
$ kubectl get secret -n <namespace> <quayregistry-name>-quay-config-secret-22dbk9btcg -o jsonpath="{.data['config\.yaml']}" | base64 --decode
```

## Managed Components

The Quay Operator is capable of managing the lifecycle of many of Quay's dependencies, called _components_. Some examples are the main database, object storage, image security scanning, and more. Each of these components usually include relevant config fields in `config.yaml`. When the Operator is managing a component, it will populate the necessary config fields for you. If you choose to use an unmanaged component (provide your own database, for example), then you are responsible for providing the necessary config fields in the `config.yaml`. 

## Configuring Quay 

### Zero-Config, Batteries-Included

If you opt to have all components of Quay fully managed by the Operator and desire no additional configuration changes, you can omit the `spec.configBundleSecret` field of the `QuayRegistry` (in fact, all fields of the `spec` are optional). The Operator will generate a `Secret` for you containing a bundle of `config.yaml` (with all component fields) and TLS key/cert pair (self-signed). The `spec.configBundleSecret` field will then be auto-populated by the Operator during reconciliation.

Note that previous config `Secrets` will not be deleted by the Operator after reconfiguration. They are kept around in case of misconfiguration and a rollback to a previous configuration is necessary to restore the registry service.

### Quay Config Editor

Quay includes a standalone web application for configuration. The Quay Operator will create a Kubernetes `Service` and expose it at the URL specified in `status.configEditorEndpoint` on the `QuayRegistry` after creation. Access it using a web browser.

#### Config Editor Credentials

The password for the config editor is randomly generated during every reconcile. The username/password are contained in a `Secret` in the same namespace referenced in `status.configEditorCredentialsSecret` on the `QuayRegistry` object.

#### Reconfiguring Quay 

Once you have finished making changes to the Quay config using the editor, click the button to validate your changes. If it passes validation, click the button **Reconfigure Quay**. This will send your changes to an HTTP server ran by the Quay Operator, which will create a new `Secret` from the config bundle, and change the `spec.configBundleSecret` field on the `QuayRegistry` to reference it. This will kick off a normal reconcile loop. Once completed, Quay will now be configured with your changes.

See also the [documentation on components](https://github.com/quay/quay-operator/docs/components.md).

### Creating Config Bundle Secret

All that is needed to configure Quay using the Operator is to change the `Secret` referenced by `spec.configBundleSecret` on the `QuayRegistry` object. All the steps handled by the config editor UI flow can be replicated manually using the Kubernetes REST API. This is useful if you want to use GitOps (recommended!) and automated tooling as part of a pipeline to manage configuration changes to your Quay registry.

#### Example

1. Create a `Secret` with your configuration fields in a `config.yaml`:

`config.yaml`:
```yaml
REGISTRY_TITLE: My Awesome Quay
```

```sh
$ kubectl create secret generic --from-file config.yaml=./config.yaml test-config-bundle
```

2. Update the `QuayRegistry` to reference the created `Secret`:

`test.quayregistry.yaml`
```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: test
spec:
  configBundleSecret: test-config-bundle
```

The deployed Quay application will now use the external database.
