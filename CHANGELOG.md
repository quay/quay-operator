## Red Hat Quay Release Notes
(Red Hat Customer Portal)[https://access.redhat.com/documentation/en-us/red_hat_quay/3/html/red_hat_quay_release_notes/index]


<a name="v3.6.0"></a>
## [v3.6.0] - 2021-10-11
### Api
- [1a7e124](https://github.com/quay/quay-operator/commit/1a7e124e66c704d1170e80feea9c481462898226): removing apiextensions.k8s.io/v1beta1 references (PROJQUAY-1791)
- [8e22ded](https://github.com/quay/quay-operator/commit/8e22dedc9302a54d1b63ba433a7cde5365599910): upgrade to apiextensions.k8s.io/v1 (PROJQUAY-1791)
### Build
- [ba686da](https://github.com/quay/quay-operator/commit/ba686daad80c7148fa1f8a74c202584b785a410d): update from downstream files (PROJQUAY-2230) ([#486](https://github.com/quay/quay-operator/issues/486))
 -  [#486](https://github.com/quay/quay-operator/issues/486)### Chore
- [f3db3bd](https://github.com/quay/quay-operator/commit/f3db3bdaeb2a2f4a766929f71b8102aa10486ab0): backport build and e2e workflows (PROJQUAY-2556)
 -  [#530](https://github.com/quay/quay-operator/issues/530) -  [#535](https://github.com/quay/quay-operator/issues/535) -  [#553](https://github.com/quay/quay-operator/issues/553) -  [#557](https://github.com/quay/quay-operator/issues/557) -  [#556](https://github.com/quay/quay-operator/issues/556) -  [#559](https://github.com/quay/quay-operator/issues/559) -  [#560](https://github.com/quay/quay-operator/issues/560)- [eec4df0](https://github.com/quay/quay-operator/commit/eec4df0aecba816da25fde90d9c855721bcb317c): add QUAY_VERSION to make run command (PROJQUAY-2030)
- [079205f](https://github.com/quay/quay-operator/commit/079205fef08fe5d86011a6783cf5ba1587e72578): add QUAY_VERSION to make run command (PROJQUAY-2030)
### Clair
- [220934d](https://github.com/quay/quay-operator/commit/220934d0a49c4f31e12c5d5664151e58d188920a): point liveness probe at introspection server (PROJQUAY-1610)
### Components
- [98d4aed](https://github.com/quay/quay-operator/commit/98d4aed508a6afe6203246ac9881b6b3e64ed38c): added tls managed component (PROJQUAY-2050)
### Componentstatus
- [1ae4e3e](https://github.com/quay/quay-operator/commit/1ae4e3e77791f8d8fcc898559eeab9923deb66bb): Reporting faulty condition for quay components (PROJQUAY-1609) ([#484](https://github.com/quay/quay-operator/issues/484))
 -  [#484](https://github.com/quay/quay-operator/issues/484)### Database
- [deb5e1d](https://github.com/quay/quay-operator/commit/deb5e1d67b0230a303124170883268923acce6f2): avoid redeploy postgres during reconcile (PROJQUAY-2603) ([#543](https://github.com/quay/quay-operator/issues/543))
 -  [#543](https://github.com/quay/quay-operator/issues/543)- [cf46e87](https://github.com/quay/quay-operator/commit/cf46e87c3daba6c85b84bdbb6a2c53c42cc76a6e): avoid regenerating password (PROJQUAY-2319)
- [c76bd07](https://github.com/quay/quay-operator/commit/c76bd07e2105679cb103e3a0de6c55d65d61dc32): prefer user provided database config (PROJQUAY-2415)
### Docs
- [e9bfd42](https://github.com/quay/quay-operator/commit/e9bfd42cd43b4b71eabdaa42b2be51ee448e9fdd): add development docs for quayio branch (PROJQUAY-2015)
### Finalizer
- [3db3a10](https://github.com/quay/quay-operator/commit/3db3a105c43d62bf2d733002cb771cf2dfd793c8): check permissions before finalizing (PROJQUAY-1937)
### Fix(Bundle)
- [de26800](https://github.com/quay/quay-operator/commit/de26800a687a541a8c441c65481d7f0317bc694c): use correct channel and operator name in subscription (PROJQUAY-2556) ([#524](https://github.com/quay/quay-operator/issues/524))
 -  [#524](https://github.com/quay/quay-operator/issues/524)### Kustomize
- [c0b0d3e](https://github.com/quay/quay-operator/commit/c0b0d3e8aa23089d32fcc957ca5cc8361243171c): use Job to run database migrations (PROJQUAY-2121)
- [a749781](https://github.com/quay/quay-operator/commit/a74978160b9b05f07cd5f02f70d124fd7115b9c4): unblock rollout from Clair init (PROJQUAY-1610)
- [d363a79](https://github.com/quay/quay-operator/commit/d363a79fa8885993aa6c29751923d6cdfaebcee7): fix missing TLS cert/key in config editor (PROJQUAY-2026)
- [ac227f7](https://github.com/quay/quay-operator/commit/ac227f76b61c6965026c1140245d62c95a3e6b90): remove probes from Postgres pods (PROJQUAY-2010)
- [49a524f](https://github.com/quay/quay-operator/commit/49a524ffc17890fe23662409a59101672934206f): fix unamanaged Postgres component (PROJQUAY-2002)
- [1c6d3f1](https://github.com/quay/quay-operator/commit/1c6d3f1b45abe66fa471b610d70af48f033f6bbb): add HorizontalPodAutoscaler to Clair+Mirror (PROJQUAY-1449)
- [6cc7f71](https://github.com/quay/quay-operator/commit/6cc7f71dec51246310ade04ebed11d93ad750374): persist DB_URI for managed postgres (PROJQUAY-1635)
### Merge Branch 'Redhat-3.6' Of Https
- [d55c297](https://github.com/quay/quay-operator/commit/d55c2971af5a444c261b0cbcc49faaca98109156): //github.com/quay/quay-operator into redhat-3.6
### Migration
- [9132827](https://github.com/quay/quay-operator/commit/91328271b806c2635e9bf0fb850d407d7f2505d2): using edge route if tls type is none (PROJQUAY-2611) ([#555](https://github.com/quay/quay-operator/issues/555))
 -  [#555](https://github.com/quay/quay-operator/issues/555)- [30eefd7](https://github.com/quay/quay-operator/commit/30eefd73950ae74f000d60953b13c65941e3c386): moving strategy to Recreate before upgrading (PROJQUAY-2586)
### Mirror
- [f81d919](https://github.com/quay/quay-operator/commit/f81d919a32e4eb6682174d1a6f774e3dfd6c262c): Set mirror as managed when flag enabled in editor (PROJQUAY-2489) ([#531](https://github.com/quay/quay-operator/issues/531))
 -  [#531](https://github.com/quay/quay-operator/issues/531)- [c003063](https://github.com/quay/quay-operator/commit/c00306350b326f3d04661500c817e82aff4ab138): Set mirror as managed when flag enabled in editor (PROJQUAY-2489) ([#531](https://github.com/quay/quay-operator/issues/531))
 -  [#531](https://github.com/quay/quay-operator/issues/531)### Mirrorprobes
- [8b94d4b](https://github.com/quay/quay-operator/commit/8b94d4bd18d4c6845e7bc6501e4611d9851c455f): removing mirror pod probes (PROJQUAY-2226) ([#485](https://github.com/quay/quay-operator/issues/485))
 -  [#485](https://github.com/quay/quay-operator/issues/485)### Objectbucketclaim
- [0e52810](https://github.com/quay/quay-operator/commit/0e52810ee273b1dbd5c97b91423bf3650ceec7d7): update lib-bucket-provisioner module (PROJQUAY-2051)
### Postgres
- [fed4453](https://github.com/quay/quay-operator/commit/fed4453be85bc9cc5dc2223e3d9c74c1b31f5955): giving postgres room to graceful shutdown (PROJQUAY-2319)
### Quay-Operator
- [5764995](https://github.com/quay/quay-operator/commit/576499505a6d292c2393ed52bbf81801fa6b9a67): advertise disconnected support (PROJQUAY-2391)
- [b596f12](https://github.com/quay/quay-operator/commit/b596f12a0cf45759f7e0e71fb4073172c7803853): add resource requests and limits (PROJQUAY-2011)
### Reconcile
- [ba68643](https://github.com/quay/quay-operator/commit/ba686431e0971bd60eebe4adafe76380a840f4fb): Prevent unnecessary component enabling/disabling (PROJQUAY-2198)
- [ad9d95e](https://github.com/quay/quay-operator/commit/ad9d95e8f0c1508f9da906a6c06d431243ea3a76): scale deployment to zero during all upgrades (PROJQUAY-2121)
### Redis
- [931e812](https://github.com/quay/quay-operator/commit/931e812097204554cfc87f3a78b0ad2421db90e9): Mark Redis as a required component (PROJQUAY-2455) ([#536](https://github.com/quay/quay-operator/issues/536))
 -  [#536](https://github.com/quay/quay-operator/issues/536)### Route
- [654f2c5](https://github.com/quay/quay-operator/commit/654f2c533824a120fb6b57b4158415b5d6b2dc54): Make sure router name is removed from cluster hostname in OCP 4.8 (PROJQUAY-2306)
### Status
- [c0292e9](https://github.com/quay/quay-operator/commit/c0292e90a486480f78c29299ce49ce3bd8f0187a): omit conflict errors (PROJQUAY-2610)
- [311688e](https://github.com/quay/quay-operator/commit/311688ec5c7fb14b5b86bfaae2081fba356e9350): Only check for object bucket claim when object storage is managed (PROJQUAY-0000)
### Tls
- [488bc4e](https://github.com/quay/quay-operator/commit/488bc4e849b9ef89c12db943850271c219a80a65): Remove tls certificates from reonciled secret (PROJQUAY-2606)
- [fd5ea72](https://github.com/quay/quay-operator/commit/fd5ea72159672fb0714a351890ce3ed3430bbd44): mounting config tls under extra_ca_certs (PROJQUAY-2575)
- [ea86fb1](https://github.com/quay/quay-operator/commit/ea86fb18da8331313290ed4f4269b1dbf76bdd38): mounting config tls under extra_ca_certs (PROJQUAY-2575)
- [be0f36f](https://github.com/quay/quay-operator/commit/be0f36f29614e9612a28324e660a865ab4bb9330): executing pod termination (PROJQUAY-2428) ([#517](https://github.com/quay/quay-operator/issues/517))
 -  [#517](https://github.com/quay/quay-operator/issues/517)- [dc31182](https://github.com/quay/quay-operator/commit/dc31182dcfc418a8182bace791bad96fa63eab15): Check for certs to mark tls as unmanaged (PROJQUAY-2348)
- [9507d22](https://github.com/quay/quay-operator/commit/9507d221fd88139e6847be698d153500dd4a51e8): persist generated TLS cert/key pair (PROJQUAY-1838)
### Tlscerts
- [80b92b6](https://github.com/quay/quay-operator/commit/80b92b698836e2e7a2269e683da29c3ddd3b4f70): keep old config bundle properties (PROJQUAY-2419)
### Tlscomponent
- [55c03ae](https://github.com/quay/quay-operator/commit/55c03ae723187919105480f62faf166ccb8ff67b): changing TLS management state evaluation (PROJQUAY-2428)
### Ui
- [925f26a](https://github.com/quay/quay-operator/commit/925f26ada89f6639651c728dc114794a044f9d47): Add tls component to Openshift Console (PROJQUAY-2308) ([#491](https://github.com/quay/quay-operator/issues/491))
 -  [#491](https://github.com/quay/quay-operator/issues/491)### Upgrade
- [0e563be](https://github.com/quay/quay-operator/commit/0e563be54e1719f17952ce9200a62602e7a736a4): Upgrade rbac version to v1 (PROJQUAY-2516)
- [9a556c1](https://github.com/quay/quay-operator/commit/9a556c17749125c4792ecbcacb9431a1e43d361f): making go routine resilient to conflicts (PROJQUAY-2395)
### Upgrades
- [b9b91c0](https://github.com/quay/quay-operator/commit/b9b91c026096b842558c73cb07261fd3173b5261): Fix CRD schema validation during upgrade (PROJQUAY-2587) ([#541](https://github.com/quay/quay-operator/issues/541))
 -  [#541](https://github.com/quay/quay-operator/issues/541)### Pull Requests
- Merge pull request [#519](https://github.com/quay/quay-operator/issues/519) from quay/PROJQUAY-2516
- Merge pull request [#503](https://github.com/quay/quay-operator/issues/503) from quay/PROJQUAY-2306
- Merge pull request [#504](https://github.com/quay/quay-operator/issues/504) from quay/obc_check
- Merge pull request [#488](https://github.com/quay/quay-operator/issues/488) from quay/fix_component_switching
- Merge pull request [#462](https://github.com/quay/quay-operator/issues/462) from dmesser/resource-requests-limits
- Merge pull request [#471](https://github.com/quay/quay-operator/issues/471) from ricardomaraschini/apiextensions-v1
- Merge pull request [#475](https://github.com/quay/quay-operator/issues/475) from alecmerdler/PROJQUAY-2121
- Merge pull request [#470](https://github.com/quay/quay-operator/issues/470) from alecmerdler/PROJQUAY-2121
- Merge pull request [#469](https://github.com/quay/quay-operator/issues/469) from alecmerdler/PROJQUAY-2050
- Merge pull request [#457](https://github.com/quay/quay-operator/issues/457) from alecmerdler/PROJQUAY-1610
- Merge pull request [#468](https://github.com/quay/quay-operator/issues/468) from alecmerdler/PROJQUAY-2026
- Merge pull request [#466](https://github.com/quay/quay-operator/issues/466) from alecmerdler/PROJQUAY-2051
- Merge pull request [#464](https://github.com/quay/quay-operator/issues/464) from alecmerdler/quayio
- Merge pull request [#463](https://github.com/quay/quay-operator/issues/463) from alecmerdler/make-run-command
- Merge pull request [#453](https://github.com/quay/quay-operator/issues/453) from alecmerdler/PROJQUAY-1838
- Merge pull request [#461](https://github.com/quay/quay-operator/issues/461) from alecmerdler/PROJQUAY-2010
- Merge pull request [#454](https://github.com/quay/quay-operator/issues/454) from quay/PROJQUAY-1791
- Merge pull request [#460](https://github.com/quay/quay-operator/issues/460) from alecmerdler/quayio-dev-docs
- Merge pull request [#458](https://github.com/quay/quay-operator/issues/458) from alecmerdler/PROJQUAY-2002
- Merge pull request [#455](https://github.com/quay/quay-operator/issues/455) from alecmerdler/PROJQUAY-1937
- Merge pull request [#452](https://github.com/quay/quay-operator/issues/452) from alecmerdler/PROJQUAY-1449
- Merge pull request [#451](https://github.com/quay/quay-operator/issues/451) from alecmerdler/PROJQUAY-1635
- Merge pull request [#447](https://github.com/quay/quay-operator/issues/447) from quay/hank/liveness


<a name="v3.6.0-alpha.4"></a>
## [v3.6.0-alpha.4] - 2021-04-23
### Chore
- [87f7900](https://github.com/quay/quay-operator/commit/87f7900bfcfa484ee6d1796e504b049d5b9272cb): v3.6.0-alpha.4 changelog bump (PROJQUAY-1486)
- [9ce1df7](https://github.com/quay/quay-operator/commit/9ce1df7fcd5055ac39d70ebb7fccd97c94b0e6e4): set quay and clair releases (PROJQUAY-1486)
- [0b22f4d](https://github.com/quay/quay-operator/commit/0b22f4dd5cc449d655526bdd4d8f8bfbda5873ca): fix chglog params (PROJQUAY-1486)
- [5b4bf55](https://github.com/quay/quay-operator/commit/5b4bf556135a301a9b9c34f4553e56cc606634e6): correct version sent to prepare-release (PROJQUAY-1486)
- [337ac92](https://github.com/quay/quay-operator/commit/337ac924207b870f46c74fcb0ddddf793c4625b7): correct version sent to prepare-release (PROJQUAY-1486)
- [ea63cf5](https://github.com/quay/quay-operator/commit/ea63cf514330f8c914eca320d22d59aa58786d39): correct version sent to prepare-release (PROJQUAY-1486)
- [a9c6687](https://github.com/quay/quay-operator/commit/a9c6687e4a7517a0eed0f7843551e0e7048ba886): prepare-release update csv (PROJQUAY-1486)
### Deps
- [ef4e0de](https://github.com/quay/quay-operator/commit/ef4e0dead7c50a7b41d264230a12c4ce1cd634f6): update controller-runtime to v0.8.2 (PROJQUAY-1622)
### Feature
- [c58d804](https://github.com/quay/quay-operator/commit/c58d80472d7a2bab69ccf0dc18f0049d115201aa): Allow image tags to be used in place of digest (PROJQUAY-1890)
### Kustomize
- [558f167](https://github.com/quay/quay-operator/commit/558f1670eec919859b0f6ec5bf2918bdc6e8d482): prevent race conditions by sorting k8s objects before creation (PROJQUAY-1915)
- [e2978f5](https://github.com/quay/quay-operator/commit/e2978f591a08126ca0d764fafc8e135f02a4e273): add ServiceAccounts for managed components (PROJQUAY-1909)
- [51859ca](https://github.com/quay/quay-operator/commit/51859ca093ffd8b743e78fa2ccdfda344f5dee28): use separate ServiceAccount for Quay app pods (PROJQUAY-1909)
### Pull Requests
- Merge pull request [#450](https://github.com/quay/quay-operator/issues/450) from quay/ready-v3.6.0-alpha.4
- Merge pull request [#449](https://github.com/quay/quay-operator/issues/449) from thomasmckay/1486-prepare-release
- Merge pull request [#448](https://github.com/quay/quay-operator/issues/448) from thomasmckay/1486-fix-makefile-4
- Merge pull request [#446](https://github.com/quay/quay-operator/issues/446) from alecmerdler/PROJQUAY-1915
- Merge pull request [#445](https://github.com/quay/quay-operator/issues/445) from alecmerdler/PROJQUAY-1909
- Merge pull request [#444](https://github.com/quay/quay-operator/issues/444) from alecmerdler/PROJQUAY-1909
- Merge pull request [#443](https://github.com/quay/quay-operator/issues/443) from thomasmckay/1486-fix-makefile-3
- Merge pull request [#441](https://github.com/quay/quay-operator/issues/441) from thomasmckay/1486-fix-makefile-2
- Merge pull request [#439](https://github.com/quay/quay-operator/issues/439) from thomasmckay/1486-fix-makefile
- Merge pull request [#433](https://github.com/quay/quay-operator/issues/433) from thomasmckay/1486-prepare-release-bundle
- Merge pull request [#436](https://github.com/quay/quay-operator/issues/436) from jonathankingfc/allow_image_tag
- Merge pull request [#434](https://github.com/quay/quay-operator/issues/434) from alecmerdler/update-controller-runtime


<a name="v3.6.0-alpha.3"></a>
## [v3.6.0-alpha.3] - 2021-04-15
### Chore
- [7581dda](https://github.com/quay/quay-operator/commit/7581dda81098d1b229dbb120004d7dcb7d18d9a2): v3.6.0-alpha.3 changelog bump (PROJQUAY-1486)
- [3a34acd](https://github.com/quay/quay-operator/commit/3a34acdb320283e6e4ffbd1da4e35fde8f93c735): fix release actions (PROJQUAY-1486)
- [98626d9](https://github.com/quay/quay-operator/commit/98626d9a6b428b61083e554494f71cbb5aaeeb9a): fix release actions (PROJQUAY-1486)
### Pull Requests
- Merge pull request [#432](https://github.com/quay/quay-operator/issues/432) from quay/ready-v3.6.0-alpha.3
- Merge pull request [#431](https://github.com/quay/quay-operator/issues/431) from thomasmckay/1486-cut-release-3
- Merge pull request [#429](https://github.com/quay/quay-operator/issues/429) from thomasmckay/1486-cut-release-2


<a name="v3.6.0-alpha.2"></a>
## [v3.6.0-alpha.2] - 2021-04-15
### Chore
- [4a611b2](https://github.com/quay/quay-operator/commit/4a611b222002e0aa4dd5b9a32d6664f2385905ae): fix prepare-release (PROJQUAY-1486)
### Pull Requests
- Merge pull request [#428](https://github.com/quay/quay-operator/issues/428) from thomasmckay/1486-release-1


[Unreleased]: https://github.com/quay/quay-operator/compare/v3.6.0...HEAD
[v3.6.0]: https://github.com/quay/quay-operator/compare/v3.6.0-alpha.4...v3.6.0
[v3.6.0-alpha.4]: https://github.com/quay/quay-operator/compare/v3.6.0-alpha.3...v3.6.0-alpha.4
[v3.6.0-alpha.3]: https://github.com/quay/quay-operator/compare/v3.6.0-alpha.2...v3.6.0-alpha.3
[v3.6.0-alpha.2]: https://github.com/quay/quay-operator/compare/v3.6.0-alpha.1...v3.6.0-alpha.2
