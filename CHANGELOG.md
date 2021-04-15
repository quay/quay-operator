## Red Hat Quay Release Notes
(Red Hat Customer Portal)[https://access.redhat.com/documentation/en-us/red_hat_quay/3/html/red_hat_quay_release_notes/index]


<a name="v3.6.0-alpha.2"></a>
## v3.6.0-alpha.2 - 2021-04-15
### Add 'Https
- [8a4ce4c](https://github.com/quay/quay-operator/commit/8a4ce4c5e8f1f0c9c64cd8b3116921bb0de58ceb): //' to 'status.registryEndpoint'
### Chore
- [4a611b2](https://github.com/quay/quay-operator/commit/4a611b222002e0aa4dd5b9a32d6664f2385905ae): fix prepare-release (PROJQUAY-1486)
- [4cd1c9b](https://github.com/quay/quay-operator/commit/4cd1c9bf24f4af25ce688efe2f2bdb57977f2d55): setup release github actions (PROJQUAY-1468)
### Kustomize
- [c944209](https://github.com/quay/quay-operator/commit/c9442099971875c7e67000e59edffd7bb94ca484): add clairctl to default allowed issuers
### Override Kustomize Using DesiredVersion
- [eba2bea](https://github.com/quay/quay-operator/commit/eba2bea7facd8db37242324ec495dac6d86907d4): dev
### PROJQUAY-1577
- [08aa698](https://github.com/quay/quay-operator/commit/08aa6982feeb1915f6c1b10004e2ea2501b47fc9): Fixed certs being overwritten when BUILDMAN_HOSTNAME is not provided
### PROJQUAY-880
- [c8db19b](https://github.com/quay/quay-operator/commit/c8db19bc503845a47f468a02427fc25bce6aaf08): Add monitoring component to Quay operator
### Postgres
- [1485d65](https://github.com/quay/quay-operator/commit/1485d6525489f25b1e3ce122a46577cf05fe7122): improve startupProbe to prevent crash looping (PROJQUAY-1664)
### WIP
- [7c2688f](https://github.com/quay/quay-operator/commit/7c2688f9f4ad0811f58d2f4447c606b6ee5532ec): Corrected multiple issues found during testing ([#114](https://github.com/quay/quay-operator/issues/114))
 -  [#114](https://github.com/quay/quay-operator/issues/114)### Reverts
- Fixed default override for quay component in docs

### Pull Requests
- Merge pull request [#428](https://github.com/quay/quay-operator/issues/428) from thomasmckay/1486-release-1
- Merge pull request [#424](https://github.com/quay/quay-operator/issues/424) from thomasmckay/1486-github-actions
- Merge pull request [#427](https://github.com/quay/quay-operator/issues/427) from alecmerdler/PROJQUAY-1664
- Merge pull request [#425](https://github.com/quay/quay-operator/issues/425) from alecmerdler/postgres-serviceaccount
- Merge pull request [#422](https://github.com/quay/quay-operator/issues/422) from alecmerdler/PROJQUAY-1797
- Merge pull request [#420](https://github.com/quay/quay-operator/issues/420) from quay/monitoring_fix
- Merge pull request [#417](https://github.com/quay/quay-operator/issues/417) from alecmerdler/fix-upstream-version
- Merge pull request [#416](https://github.com/quay/quay-operator/issues/416) from alecmerdler/operator-bundle
- Merge pull request [#415](https://github.com/quay/quay-operator/issues/415) from alecmerdler/branding-environment-variable
- Merge pull request [#406](https://github.com/quay/quay-operator/issues/406) from alecmerdler/PROJQUAY-1737
- Merge pull request [#411](https://github.com/quay/quay-operator/issues/411) from syed/fix-namespace-permission-projquay-880
- Merge pull request [#409](https://github.com/quay/quay-operator/issues/409) from syed/fix-namespace-permission-projquay-880
- Merge pull request [#408](https://github.com/quay/quay-operator/issues/408) from quay/fix_version
- Merge pull request [#407](https://github.com/quay/quay-operator/issues/407) from thomasmckay/1489-downstream
- Merge pull request [#401](https://github.com/quay/quay-operator/issues/401) from syed/projquay-880-monitoring
- Merge pull request [#405](https://github.com/quay/quay-operator/issues/405) from syed/projquay-880-add-finalizer
- Merge pull request [#404](https://github.com/quay/quay-operator/issues/404) from jonathankingfc/PROJQUAY-1577
- Merge pull request [#403](https://github.com/quay/quay-operator/issues/403) from jonathankingfc/master
- Merge pull request [#399](https://github.com/quay/quay-operator/issues/399) from alecmerdler/remove-scc-readme
- Merge pull request [#397](https://github.com/quay/quay-operator/issues/397) from alecmerdler/context-refactor
- Merge pull request [#394](https://github.com/quay/quay-operator/issues/394) from alecmerdler/PROJQUAY-1574
- Merge pull request [#392](https://github.com/quay/quay-operator/issues/392) from alecmerdler/PROJQUAY-1575
- Merge pull request [#391](https://github.com/quay/quay-operator/issues/391) from alecmerdler/catalogsource-bump
- Merge pull request [#390](https://github.com/quay/quay-operator/issues/390) from alecmerdler/fix-buildman-route-cert
- Merge pull request [#389](https://github.com/quay/quay-operator/issues/389) from alecmerdler/update-catalogsource
- Merge pull request [#387](https://github.com/quay/quay-operator/issues/387) from alecmerdler/PROJQUAY-1442
- Merge pull request [#386](https://github.com/quay/quay-operator/issues/386) from alecmerdler/PROJQUAY-1424
- Merge pull request [#377](https://github.com/quay/quay-operator/issues/377) from thomasmckay/osbs-update
- Merge pull request [#383](https://github.com/quay/quay-operator/issues/383) from alecmerdler/PROJQUAY-1395
- Merge pull request [#382](https://github.com/quay/quay-operator/issues/382) from alecmerdler/quayregistry-e2e
- Merge pull request [#379](https://github.com/quay/quay-operator/issues/379) from alecmerdler/update-catalogsource
- Merge pull request [#378](https://github.com/quay/quay-operator/issues/378) from alecmerdler/PROJQUAY-1385
- Merge pull request [#374](https://github.com/quay/quay-operator/issues/374) from alecmerdler/PROJQUAY-1345
- Merge pull request [#376](https://github.com/quay/quay-operator/issues/376) from alecmerdler/PROJQUAY-1381
- Merge pull request [#372](https://github.com/quay/quay-operator/issues/372) from alecmerdler/PROJQUAY-1306
- Merge pull request [#371](https://github.com/quay/quay-operator/issues/371) from quay/clair-config
- Merge pull request [#370](https://github.com/quay/quay-operator/issues/370) from alecmerdler/PROJQUAY-1339
- Merge pull request [#369](https://github.com/quay/quay-operator/issues/369) from alecmerdler/update-catalogsource
- Merge pull request [#368](https://github.com/quay/quay-operator/issues/368) from alecmerdler/ci-build
- Merge pull request [#367](https://github.com/quay/quay-operator/issues/367) from alecmerdler/PROJQUAY-1323
- Merge pull request [#357](https://github.com/quay/quay-operator/issues/357) from alecmerdler/PROJQUAY-869
- Merge pull request [#365](https://github.com/quay/quay-operator/issues/365) from alecmerdler/PROJQUAY-1285
- Merge pull request [#366](https://github.com/quay/quay-operator/issues/366) from thomasmckay/1177-branding
- Merge pull request [#364](https://github.com/quay/quay-operator/issues/364) from alecmerdler/PROJQUAY-1267
- Merge pull request [#363](https://github.com/quay/quay-operator/issues/363) from alecmerdler/PROJQUAY-1144
- Merge pull request [#362](https://github.com/quay/quay-operator/issues/362) from alecmerdler/PROJQUAY-1281
- Merge pull request [#361](https://github.com/quay/quay-operator/issues/361) from alecmerdler/PROJQUAY-1278
- Merge pull request [#360](https://github.com/quay/quay-operator/issues/360) from alecmerdler/PROJQUAY-1268
- Merge pull request [#359](https://github.com/quay/quay-operator/issues/359) from alecmerdler/PROJQUAY-1267
- Merge pull request [#356](https://github.com/quay/quay-operator/issues/356) from alecmerdler/update-catalogsource
- Merge pull request [#355](https://github.com/quay/quay-operator/issues/355) from alecmerdler/fix-reconfigure-debug
- Merge pull request [#354](https://github.com/quay/quay-operator/issues/354) from alecmerdler/PROJQUAY-1156
- Merge pull request [#353](https://github.com/quay/quay-operator/issues/353) from BillDett/PROJQUAY-1202
- Merge pull request [#352](https://github.com/quay/quay-operator/issues/352) from alecmerdler/postgres-fsgroup
- Merge pull request [#351](https://github.com/quay/quay-operator/issues/351) from alecmerdler/PROJQUAY-1240
- Merge pull request [#349](https://github.com/quay/quay-operator/issues/349) from thomasmckay/839-disconnected
- Merge pull request [#350](https://github.com/quay/quay-operator/issues/350) from alecmerdler/PROJQUAY-1239
- Merge pull request [#326](https://github.com/quay/quay-operator/issues/326) from alecmerdler/conditions
- Merge pull request [#348](https://github.com/quay/quay-operator/issues/348) from thomasmckay/1236-debug
- Merge pull request [#346](https://github.com/quay/quay-operator/issues/346) from thomasmckay/1157-rados-rhocs
- Merge pull request [#345](https://github.com/quay/quay-operator/issues/345) from jonathankingfc/debug_log_default
- Merge pull request [#347](https://github.com/quay/quay-operator/issues/347) from BillDett/fix_override_doc
- Merge pull request [#332](https://github.com/quay/quay-operator/issues/332) from thomasmckay/340-manifests
- Merge pull request [#344](https://github.com/quay/quay-operator/issues/344) from alecmerdler/PROJQUAY-1201
- Merge pull request [#343](https://github.com/quay/quay-operator/issues/343) from alecmerdler/image-overrides
- Merge pull request [#342](https://github.com/quay/quay-operator/issues/342) from alecmerdler/PROJQUAY-1196
- Merge pull request [#341](https://github.com/quay/quay-operator/issues/341) from alecmerdler/quayecosystem
- Merge pull request [#340](https://github.com/quay/quay-operator/issues/340) from alecmerdler/PROJQUAY-1185
- Merge pull request [#339](https://github.com/quay/quay-operator/issues/339) from alecmerdler/repomirror-nomigrate
- Merge pull request [#337](https://github.com/quay/quay-operator/issues/337) from alecmerdler/clair-psk-marshal-fix
- Merge pull request [#338](https://github.com/quay/quay-operator/issues/338) from alecmerdler/route-custom-host-rbac
- Merge pull request [#337](https://github.com/quay/quay-operator/issues/337) from alecmerdler/clair-psk-marshal-fix
- Merge pull request [#336](https://github.com/quay/quay-operator/issues/336) from jonathankingfc/fix-arg
- Merge pull request [#335](https://github.com/quay/quay-operator/issues/335) from quay/ct-to-quay-container
- Merge pull request [#334](https://github.com/quay/quay-operator/issues/334) from alecmerdler/fix-objectstorage
- Merge pull request [#333](https://github.com/quay/quay-operator/issues/333) from alecmerdler/managed-components-docs
- Merge pull request [#325](https://github.com/quay/quay-operator/issues/325) from alecmerdler/PROJQUAY-1107
- Merge pull request [#319](https://github.com/quay/quay-operator/issues/319) from alecmerdler/PROJQUAY-828
- Merge pull request [#329](https://github.com/quay/quay-operator/issues/329) from alecmerdler/quayecosystem
- Merge pull request [#331](https://github.com/quay/quay-operator/issues/331) from alecmerdler/PROJQUAY-992
- Merge pull request [#330](https://github.com/quay/quay-operator/issues/330) from alecmerdler/PROJQUAY-1122
- Merge pull request [#307](https://github.com/quay/quay-operator/issues/307) from alecmerdler/PROJQUAY-954
- Merge pull request [#328](https://github.com/quay/quay-operator/issues/328) from alecmerdler/multigroup
- Merge pull request [#327](https://github.com/quay/quay-operator/issues/327) from alecmerdler/tng-demo
- Merge pull request [#324](https://github.com/quay/quay-operator/issues/324) from alecmerdler/PROJQUAY-1112
- Merge pull request [#323](https://github.com/quay/quay-operator/issues/323) from alecmerdler/disable-builds
- Merge pull request [#322](https://github.com/quay/quay-operator/issues/322) from alecmerdler/PROJQUAY-1107
- Merge pull request [#308](https://github.com/quay/quay-operator/issues/308) from alecmerdler/PROJQUAY-1065
- Merge pull request [#296](https://github.com/quay/quay-operator/issues/296) from thomasmckay/v2-readme
- Merge pull request [#321](https://github.com/quay/quay-operator/issues/321) from alecmerdler/external-access-docs
- Merge pull request [#320](https://github.com/quay/quay-operator/issues/320) from alecmerdler/PROJQUAY-1091
- Merge pull request [#318](https://github.com/quay/quay-operator/issues/318) from alecmerdler/PROJQUAY-1103
- Merge pull request [#317](https://github.com/quay/quay-operator/issues/317) from alecmerdler/PROJQUAY-1102
- Merge pull request [#315](https://github.com/quay/quay-operator/issues/315) from alecmerdler/PROJQUAY-1091
- Merge pull request [#314](https://github.com/quay/quay-operator/issues/314) from alecmerdler/PROJQUAY-992
- Merge pull request [#313](https://github.com/quay/quay-operator/issues/313) from alecmerdler/vader
- Merge pull request [#306](https://github.com/quay/quay-operator/issues/306) from alecmerdler/PROJQUAY-909-endpoint
- Merge pull request [#312](https://github.com/quay/quay-operator/issues/312) from alecmerdler/PROJQUAY-1087
- Merge pull request [#311](https://github.com/quay/quay-operator/issues/311) from alecmerdler/update-dockerfile
- Merge pull request [#303](https://github.com/quay/quay-operator/issues/303) from alecmerdler/update-catalogsource
- Merge pull request [#302](https://github.com/quay/quay-operator/issues/302) from alecmerdler/install-instructions
- Merge pull request [#297](https://github.com/quay/quay-operator/issues/297) from alecmerdler/PROJQUAY-909
- Merge pull request [#298](https://github.com/quay/quay-operator/issues/298) from alecmerdler/PROJQUAY-932
- Merge pull request [#300](https://github.com/quay/quay-operator/issues/300) from alecmerdler/PROJQUAY-860
- Merge pull request [#299](https://github.com/quay/quay-operator/issues/299) from alecmerdler/PROJQUAY-853
- Merge pull request [#291](https://github.com/quay/quay-operator/issues/291) from alecmerdler/pull-request-template
- Merge pull request [#290](https://github.com/quay/quay-operator/issues/290) from alecmerdler/PROJQUAY-952
- Merge pull request [#289](https://github.com/quay/quay-operator/issues/289) from alecmerdler/PROJQUAY-865
- Merge pull request [#288](https://github.com/quay/quay-operator/issues/288) from alecmerdler/operatorgroup
- Merge pull request [#287](https://github.com/quay/quay-operator/issues/287) from alecmerdler/PROJQUAY-930
- Merge pull request [#286](https://github.com/quay/quay-operator/issues/286) from alecmerdler/PROJQUAY-830
- Merge pull request [#285](https://github.com/quay/quay-operator/issues/285) from alecmerdler/PROJQUAY-896
- Merge pull request [#284](https://github.com/quay/quay-operator/issues/284) from alecmerdler/fix-polling
- Merge pull request [#283](https://github.com/quay/quay-operator/issues/283) from alecmerdler/PROJQUAY-870
- Merge pull request [#282](https://github.com/quay/quay-operator/issues/282) from alecmerdler/PROJQUAY-908
- Merge pull request [#281](https://github.com/quay/quay-operator/issues/281) from alecmerdler/PROJQUAY-887
- Merge pull request [#280](https://github.com/quay/quay-operator/issues/280) from alecmerdler/components
- Merge pull request [#279](https://github.com/quay/quay-operator/issues/279) from alecmerdler/PROJQUAY-885
- Merge pull request [#277](https://github.com/quay/quay-operator/issues/277) from alecmerdler/PROJQUAY-886
- Merge pull request [#276](https://github.com/quay/quay-operator/issues/276) from alecmerdler/secret-key-generation
- Merge pull request [#275](https://github.com/quay/quay-operator/issues/275) from alecmerdler/fix-e2e-tests
- Merge pull request [#274](https://github.com/quay/quay-operator/issues/274) from alecmerdler/PROJQUAY-871
- Merge pull request [#273](https://github.com/quay/quay-operator/issues/273) from alecmerdler/PROJQUAY-867
- Merge pull request [#272](https://github.com/quay/quay-operator/issues/272) from alecmerdler/PROJQUAY-866
- Merge pull request [#271](https://github.com/quay/quay-operator/issues/271) from alecmerdler/PROJQUAY-858
- Merge pull request [#270](https://github.com/quay/quay-operator/issues/270) from alecmerdler/reconcile
- Merge pull request [#269](https://github.com/quay/quay-operator/issues/269) from alecmerdler/inflate
- Merge pull request [#268](https://github.com/quay/quay-operator/issues/268) from alecmerdler/quayregistry-api
- Merge pull request [#267](https://github.com/quay/quay-operator/issues/267) from alecmerdler/kubebuilder-scaffold
- Merge pull request [#266](https://github.com/quay/quay-operator/issues/266) from alecmerdler/kustomize-init
- Merge pull request [#265](https://github.com/quay/quay-operator/issues/265) from alecmerdler/greenfield
- Merge pull request [#263](https://github.com/quay/quay-operator/issues/263) from sabre1041/helm-test-upgrade
- Merge pull request [#238](https://github.com/quay/quay-operator/issues/238) from sabre1041/security-context
- Merge pull request [#169](https://github.com/quay/quay-operator/issues/169) from jjmengze/patch-1
- Merge pull request [#146](https://github.com/quay/quay-operator/issues/146) from sabre1041/1.0.2-release
- Merge pull request [#144](https://github.com/quay/quay-operator/issues/144) from redhat-cop/helm-release
- Merge pull request [#141](https://github.com/quay/quay-operator/issues/141) from redhat-cop/helm
- Merge pull request [#140](https://github.com/quay/quay-operator/issues/140) from sabre1041/fix-rados
- Merge pull request [#139](https://github.com/quay/quay-operator/issues/139) from sabre1041/fix-gh-actions-perms
- Merge pull request [#125](https://github.com/quay/quay-operator/issues/125) from sabre1041/k8s-native
- Merge pull request [#136](https://github.com/quay/quay-operator/issues/136) from sabre1041/rados-storage-fix
- Merge pull request [#134](https://github.com/quay/quay-operator/issues/134) from sabre1041/gh-actions-badge-fix
- Merge pull request [#129](https://github.com/quay/quay-operator/issues/129) from sabre1041/gh-actions
- Merge pull request [#123](https://github.com/quay/quay-operator/issues/123) from sabre1041/1.0.1-release
- Merge pull request [#119](https://github.com/quay/quay-operator/issues/119) from sabre1041/repomirror
- Merge pull request [#122](https://github.com/quay/quay-operator/issues/122) from sabre1041/fix-distributed-storage-options
- Merge pull request [#120](https://github.com/quay/quay-operator/issues/120) from sabre1041/fix-credentials
- Merge pull request [#118](https://github.com/quay/quay-operator/issues/118) from sabre1041/ga-release
- Merge pull request [#117](https://github.com/quay/quay-operator/issues/117) from sabre1041/storage-doc-fix
- Merge pull request [#56](https://github.com/quay/quay-operator/issues/56) from sabre1041/operator-sdk-upgrade-0.10
- Merge pull request [#48](https://github.com/quay/quay-operator/issues/48) from sabre1041/version-bump-v0.0.4
- Merge pull request [#47](https://github.com/quay/quay-operator/issues/47) from sabre1041/subscription-update-0.0.3


[Unreleased]: https://github.com/quay/quay-operator/compare/v3.6.0-alpha.2...HEAD
