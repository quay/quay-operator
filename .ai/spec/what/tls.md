# TLS

The operator manages TLS certificates for Quay's HTTPS endpoint and optionally for PostgreSQL database connections. Endpoint TLS supports three modes: managed (cluster wildcard), user-provided via config bundle, and user-provided via external secret reference. Database TLS is an independent opt-in feature for encrypting managed PostgreSQL connections.

## Behavioral Rules

### Managed TLS

1. When the `tls` component is managed, the operator uses the cluster's wildcard certificate (extracted from the probe Route's TLS dial during feature detection) as the TLS cert for Quay.
2. If no wildcard certificate can be extracted (e.g., TLS dial fails), the operator continues without it. Quay falls back to its own certificate generation.
3. The `tls` component defaults to managed when the Route API is available AND no user-provided TLS cert/key pair is found in the config bundle.

### User-Provided TLS via Config Bundle

4. Users can provide `ssl.cert` and `ssl.key` in the `configBundleSecret`. When present, these are used as Quay's TLS certificate.
5. When user-provided certs are detected in the config bundle, the `tls` component defaults to unmanaged.
6. The middleware strips `ssl.cert` and `ssl.key` from the rendered config secret so they do not end up in `/conf/stack` and interfere with Quay's NGINX config generation. The certs are mounted via a separate TLS volume.

### External TLS via SecretRef

7. When the `tls` component is unmanaged, users may reference a `kubernetes.io/tls` Secret via `secretRef` on the TLS component. The Secret must contain `tls.crt` and `tls.key`.
8. The external TLS secret is validated: both cert and key must be present and must form a valid TLS key pair.
9. The `secretRef` and `ssl.cert`/`ssl.key` in the config bundle are mutually exclusive. If both are present, the operator returns an error and blocks reconciliation.
10. The operator auto-labels the external TLS secret with `quay.redhat.com/tls-secret: "true"` so the controller-runtime cache informer watches it for changes.
11. When the external TLS secret's content changes, the operator computes a SHA-256 hash of the cert+key (last 8 hex chars) and stores it in the `QuayRegistryContext.TLSSecretHash`. This hash is propagated as an annotation on the Quay deployment, triggering a rolling restart.

### Cluster TLS Security Profile

12. On OpenShift, the operator reads the `APIServer` resource named `"cluster"` (API group `config.openshift.io/v1`) to determine the cluster-wide TLS security profile.
13. The TLS profile is translated to two formats:
    - `SSL_PROTOCOLS`: space-separated TLS version list in nginx format (e.g., `"TLSv1.2 TLSv1.3"`)
    - `SSL_CIPHERS`: colon-separated cipher suite names in OpenSSL format
14. These values are injected into Quay's config via the `QuayRegistryContext`.
15. If the user has already set `SSL_PROTOCOLS` or `SSL_CIPHERS` in their `config.yaml`, the cluster profile is NOT applied. User overrides always take precedence.
16. On vanilla Kubernetes (no `config.openshift.io` API), the TLS profile check is silently skipped.
17. A `nil` TLS profile on the APIServer resource defaults to `Intermediate` profile (TLSv1.2+).

### CA Trust Chain

18. The operator manages two CA-related ConfigMaps per QuayRegistry instance:
    - `<name>-cluster-service-ca`: Injected by the OpenShift service CA operator with the service signing CA certificate (`service-ca.crt` key).
    - `<name>-cluster-trusted-ca`: Injected by the OpenShift CA injection operator with the cluster's trusted CA bundle (`ca-bundle.crt` key).
19. The `cluster-trusted-ca` ConfigMap must contain a `ca-bundle.crt` key in its data, even if the value is empty. On KinD/vanilla Kubernetes, no CA injection operator populates this key, so the kustomize base manifest includes it with an empty value. Without this key, the Clair deployment's volume mount fails.
20. The operator tracks content hashes of both CA ConfigMaps via annotations. When the CA content changes (e.g., certificate rotation), the annotation update triggers a pod restart.
21. The middleware also strips `clair-ssl.key` and `clair-ssl.crt` from the rendered config secret, as Clair TLS certs are handled separately.

### Database TLS [PLANNED: PROJQUAY-11215]

22. Database TLS encrypts connections from Quay app → managed postgres and Clair → managed clairpostgres. It is independent of the `tls` component (Quay HTTPS endpoint) — enabling one does not require or imply the other.
23. Database TLS is opt-in via a spec field on the postgres/clairpostgres component. It is off by default. Enabling it does not affect unmanaged postgres components.
24. When database TLS is enabled, the PostgreSQL server is configured with `ssl = on`, `ssl_cert_file`, and `ssl_key_file` directives. The TLS cert/key are mounted as a volume in the postgres Deployment.
25. The PostgreSQL server cert Secret follows the naming pattern `<registry-name>-<component>-pg-tls` (e.g., `example-quay-database-pg-tls`).
26. Cert provisioning has two paths, with user-provided taking precedence:
    - **Service CA (OpenShift, default):** The operator annotates the PG Service with `service.beta.openshift.io/serving-cert-secret-name` to trigger auto-generation of a TLS Secret signed by the cluster's service CA.
    - **User-provided:** The user supplies a `kubernetes.io/tls` Secret containing the PG server cert/key, plus the corresponding CA cert for client trust.
27. When database TLS is enabled, clients connect with `sslmode=verify-ca`. The CA root cert is mounted into Quay app and Clair pods from the `cluster-service-ca` ConfigMap (service CA path) or a user-provided CA source.
28. If database TLS is enabled but no cert source is available (no service CA operator, no user-provided cert), the operator blocks reconciliation with `RolloutBlocked` condition and reason `ConfigInvalid`.
29. The CA trust chain for database TLS may differ from the Quay endpoint TLS CA. Service CA-signed PG certs are trusted via `service-ca.crt`, while the Quay endpoint may use a different CA.

## Constraints

- TLS certificate rotation via `secretRef` depends on the controller-runtime informer cache. The TLS secret must be labeled with `quay.redhat.com/tls-secret: "true"` for the watch to work.
- The probe Route for cluster hostname discovery creates and deletes a Route resource. If the Route lingers (e.g., API server is slow), subsequent reconciles wait for ingress status before proceeding.
- Database TLS on KinD/vanilla Kubernetes requires a user-provided cert — the service CA operator is OpenShift-only.
