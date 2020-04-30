1.1.0

Changes:

- Resolved issues with GitHub Actions CI/CD pipeline
  [#147](https://github.com/redhat-cop/quay-operator/pull/147)
- Enhanced logic for Quay Configuration route
  [#148](https://github.com/redhat-cop/quay-operator/pull/148) 
- Update to operator-sdk 0.15.2
  [#153](https://github.com/redhat-cop/quay-operator/pull/153)
- Quay SSL Certificate uses TLS secret type
  [#155](https://github.com/redhat-cop/quay-operator/pull/155)
- Resolved issue when specifying multiple replicas of a given component
  [#159](https://github.com/redhat-cop/quay-operator/pull/159)
- Updating example Quay Ecosystem Custom Resource examples
  [#163](https://github.com/redhat-cop/quay-operator/pull/163)
- Retrofitted how external access is specified and managed
  [#164](https://github.com/redhat-cop/quay-operator/pull/164)
- New Schema for defining externalAccess as a field in QuayEcoystem
- Support for additional external access types (LoadBalancer and Ingress) 
- Add additional roles to CSV to manage ingresses.
  [#202](https://github.com/redhat-cop/quay-operator/pull/202)
- Always use Port 8443 for Quay Config App's health probes.
  [#200](https://github.com/redhat-cop/quay-operator/pull/200)
- The Quay Config App now continues running by default.
  [#189](https://github.com/redhat-cop/quay-operator/pull/189)
- The Redis and Hostname configuration are marked "Read Only" in the Quay
  Configuration App.
  [#188](https://github.com/redhat-cop/quay-operator/pull/188)
- The "Repo Mirror" pod is now health-checked using the correct port.
  [#187](https://github.com/redhat-cop/quay-operator/pull/187)
- Support for managing superusers.
  [#187](https://github.com/redhat-cop/quay-operator/pull/187)
- Added support for injecting config files for Quay and Clair.
  [#187](https://github.com/redhat-cop/quay-operator/pull/187)
- (OpenShift) SCC management refinement. Removal of SCCs when QuayEcosystem is
  deleted through the use of finalizers.
  [#187](https://github.com/redhat-cop/quay-operator/pull/187)
- Certificates and other secrets are now mounted in a way that is compatible
  with Quay and Quay's Config App.
  [#187](https://github.com/redhat-cop/quay-operator/pull/187)
- The operator now verifies the configuration for the Hostname, Redis, and
  Postgres when Quay's configuration secret is changed.
  [#177](https://github.com/redhat-cop/quay-operator/pull/177)
- Changed the default "From" email address used by Quay.
  [#177](https://github.com/redhat-cop/quay-operator/pull/177)
- The Operator uses the latest Quay image.
  [#177](https://github.com/redhat-cop/quay-operator/pull/177)
- Fixed a spelling error in log output.
  [#169](https://github.com/redhat-cop/quay-operator/pull/169)

Known Issues:

- Configuring Storage Geo-Replication for Azure in the CR causes the deployment
  to fail. [PROJQUAY-637](https://issues.redhat.com/browse/PROJQUAY-637)
- The Hostname is set to an IP Address when using Load Balancers on GCP which
  causes the self-signed certificate validation to fail in Quay’s Config
  Application. [PROJQUAY-638](https://issues.redhat.com/browse/PROJQUAY-638)
- Using the Postgres or Redis images from Dockerhub will fail.
  [PROJQUAY-642](https://issues.redhat.com/browse/PROJQUAY-642)
  [PROJQUAY-643](https://issues.redhat.com/browse/PROJQUAY-643)
- For advanced persistance configurations, Quay's `PROXY_STORAGE` feature is
  not exposed through the CR and can only be managed through Quay's Config app.
  [PROJQUAY-612](https://issues.redhat.com/browse/PROJQUAY-612)
- Quay's Config App will always using TLS; it is not possible to configure it
  as HTTP-only in the CR.
  [PROJQUAY-631](https://issues.redhat.com/browse/PROJQUAY-631)
- Node Ports do not currently work. 
  [PROJQUAY-636](https://issues.redhat.com/browse/PROJQUAY-636)
- Cloudfront cannot be properly configured using the CR. It can be managed
  using Quay's configuration app.
  [PROJQUAY-651](https://issues.redhat.com/browse/PROJQUAY-651)
- This version of the operator cannot be used for an automatic upgrade due to
  schema changes in the CR. 
  [PROJQUAY-653](https://issues.redhat.com/browse/PROJQUAY-653)

