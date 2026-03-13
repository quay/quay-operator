# Kubernetes Operator PR Review

Perform a comprehensive review of a pull request **as an expert senior Go and Kubernetes operator engineer**. Apply rigorous code quality standards while evaluating controller-runtime patterns, CRD compatibility, reconciliation correctness, and operational safety for production Kubernetes/OpenShift environments.

## Reviewer Persona

You are reviewing this PR as a **senior staff engineer with 15+ years of experience** and deep expertise in:

**Go Backend:**
- Go 1.23+ features and idioms
- Error handling patterns (wrapping with `%w`, sentinel errors, never swallowing)
- Interface design and composition
- Goroutine safety and synchronization
- Resource cleanup (`defer`, context cancellation)
- Table-driven tests and subtests
- Package organization and naming conventions

**Kubernetes/Operator:**
- controller-runtime (Kubebuilder) reconciliation patterns
- CRD design, versioning, and backward compatibility
- RBAC marker generation and least-privilege scoping
- Finalizer lifecycle and owner references
- Status subresource updates and condition management
- Watch predicates and event filtering
- Kustomize manifest generation
- Operator Lifecycle Manager (OLM) bundle/CSV management

**OpenShift:**
- Route vs Ingress patterns
- Security Context Constraints (SCC)
- Cluster feature detection (Routes, ObjectBucketClaim, Prometheus APIs)
- Operator bundle packaging and CSV structure

**Apply rigorous senior engineer standards throughout the review.**

## PR Reference

The PR to review: `$ARGUMENTS`

---

## Phase 1: Gather PR Information

### Step 1: Fetch PR Details

```bash
gh pr view $ARGUMENTS --json title,body,files,additions,deletions,author,baseRefName,headRefName,state,labels,reviews
```

**Extract and note:**
- Title and description
- Files changed and lines modified
- Base and head branches
- Labels
- Existing review comments
- Whether the PR title follows commit format: `<subsystem>: <what changed> (PROJQUAY-####)`

### Step 2: Get Full Diff

```bash
gh pr diff $ARGUMENTS
```

### Step 3: Validate Commit Messages

Check that all commits in the PR reference a PROJQUAY Jira ticket. Every commit message should follow the format:
```
<subsystem>: <what changed> (PROJQUAY-####)
```

If any commit lacks a PROJQUAY reference, flag this as a blocking issue.

---

## Phase 2: Classify Changes

Categorize each changed file:

| Category | Path Pattern | Review Focus |
|----------|--------------|--------------|
| **CRD/API Types** | `apis/quay/v1/*.go` | Backward compatibility, field validation, deepcopy |
| **Main Reconciler** | `controllers/quay/quayregistry_controller.go` | Idempotency, requeue patterns, error handling |
| **Status Reconciler** | `controllers/quay/quayregistry_status_controller.go` | Status condition updates, polling patterns |
| **Feature Detection** | `controllers/quay/features.go` | API discovery correctness |
| **Kustomize Generation** | `pkg/kustomize/*.go` | Manifest correctness, secret handling |
| **Component Status** | `pkg/cmpstatus/*.go` | Checker interface, health evaluation |
| **Runtime Context** | `pkg/context/*.go` | State propagation, field completeness |
| **Config Validation** | `pkg/middleware/*.go` | Config schema, validation logic |
| **Kustomize Manifests** | `kustomize/**/*.yaml` | YAML structure, resource specs, security contexts |
| **OLM Bundle** | `bundle/**/*.yaml` | CSV changes, CRD schema, RBAC |
| **Tests** | `*_test.go` | Coverage, envtest usage, table-driven patterns |
| **E2E Tests** | `e2e/**` | kuttl test structure, assertions |
| **Generated Code** | `**/zz_generated.*.go` | Should be auto-generated, not hand-edited |

---

## Phase 3: CRD & API Compatibility Analysis

### Step 4: API Type Changes (if `apis/quay/v1/` modified)

**Check for BREAKING changes:**

1. **Field Removals** - Removing a field from the spec or status breaks existing CRs
   - Any field removal is a BLOCKING issue
   - Fields should be deprecated with comments, not removed

2. **Type Changes** - Changing a field's type breaks serialization
   - `string` to `int`, struct field type changes, etc.

3. **Validation Changes** - Tightening validation breaks existing CRs
   - New `+kubebuilder:validation:Required` on existing optional fields
   - New enum restrictions on existing fields
   - Reduced `+kubebuilder:validation:Maximum/Minimum` ranges

4. **Default Changes** - Changing defaults affects existing CRs without the field set

5. **DeepCopy Generation** - If types changed, was `make generate` run?
   - Check that `zz_generated.deepcopy.go` is updated consistently with type changes

6. **Component Kind Changes** - Changes to `ComponentKind` constants or `AllComponents` slice affect all downstream logic

**Generate severity assessment:**
- CRITICAL: Field removal or type change on existing fields
- HIGH: New required fields without defaults, validation tightening
- MEDIUM: Default value changes, new optional fields
- LOW: Comment changes, new types that don't affect existing CRs

---

## Phase 4: Reconciliation Logic Review

### Step 5: Reconciler Changes

For any changes to `controllers/quay/`:

**Check reconciliation correctness:**

1. **Idempotency** - Running Reconcile twice with the same input must produce the same result
   - No side effects that accumulate (duplicate resources, growing lists)
   - Create-or-update patterns use `controllerutil.CreateOrUpdate` or equivalent
   - No assumptions about prior state

2. **Requeue Patterns** - Correct use of `ctrl.Result`
   - `ctrl.Result{}` - success, no requeue (only when truly done)
   - `ctrl.Result{Requeue: true}` - immediate requeue (use sparingly)
   - `ctrl.Result{RequeueAfter: duration}` - delayed requeue (preferred for polling)
   - `return ctrl.Result{}, err` - requeue with backoff on error
   - NEVER use `time.Sleep()` or blocking polls in Reconcile

3. **Error Handling** - Proper error propagation
   - Transient errors should return `(ctrl.Result{}, err)` for exponential backoff
   - Permanent errors should set status conditions and return `(ctrl.Result{}, nil)`
   - Errors must be wrapped with context: `fmt.Errorf("doing X: %w", err)`
   - `errors.IsNotFound()` checks for expected missing resources

4. **Status Updates** - Correct status subresource usage
   - Status updates use `r.Client.Status().Update()` not `r.Client.Update()`
   - Conditions use `meta.SetStatusCondition()` for proper transitions
   - Status reflects actual state, not desired state
   - No status update in a loop without re-fetching the resource

5. **Finalizer Handling**
   - Finalizers added before external resources are created
   - Finalizer removal is the LAST step in deletion handling
   - Deletion logic handles already-deleted external resources gracefully

6. **Context Propagation**
   - `context.Context` passed through all layers (never `context.Background()` in reconcile path)
   - Context used for cancellation-aware operations
   - No goroutines spawned from Reconcile without proper lifecycle management

7. **Owner References**
   - Created resources have owner references back to QuayRegistry
   - `controllerutil.SetControllerReference()` used correctly
   - Cross-namespace ownership not attempted (not supported)

### Step 6: Feature Detection Changes

For changes to `controllers/quay/features.go`:

- API group discovery is resilient to transient failures
- Missing APIs gracefully degrade (component becomes unmanaged)
- No caching of API discovery results that could become stale

---

## Phase 5: Kustomize & Manifest Review

### Step 7: Kustomize Code Changes

For changes to `pkg/kustomize/`:

1. **Manifest Generation**
   - Kustomize overlays applied in correct order
   - Runtime values injected safely (proper YAML/JSON escaping)
   - Secret data handled securely (not logged, not in events)

2. **Resource Naming**
   - Generated resources follow naming convention: `<registry-name>-quay-<component>`
   - Labels include `quay-operator/quayregistry` for ownership

3. **Secret Handling**
   - Secrets generated with sufficient randomness
   - No secret data leaked into ConfigMaps, logs, or events
   - Secret rotation logic is correct

### Step 8: YAML Manifest Changes

For changes to `kustomize/**/*.yaml`:

1. **Deployments/StatefulSets**
   - Resource requests AND limits specified (unless `SKIP_RESOURCE_REQUESTS` dev mode)
   - Liveness and readiness probes defined
   - Image pull policy appropriate
   - `securityContext` set (non-root, read-only root filesystem where possible)
   - No `privileged: true` or unnecessary capabilities

2. **Services**
   - Correct port definitions and target ports
   - Appropriate service type (ClusterIP default)

3. **RBAC**
   - Roles scoped to minimum required permissions
   - No cluster-wide permissions when namespace-scoped suffices
   - ServiceAccount created and referenced

4. **PersistentVolumeClaims**
   - Storage size reasonable
   - Access mode appropriate
   - No hardcoded storage class (allow cluster default)

---

## Phase 6: Go Code Quality Review

### Step 9: Idiomatic Go

1. **Error Handling**
   - Errors wrapped with context: `fmt.Errorf("context: %w", err)`
   - No swallowed errors (empty `if err != nil {}` blocks)
   - Sentinel errors used for expected conditions
   - Custom error types where behavior differentiation needed

2. **Interface Design**
   - Interfaces defined where they are consumed, not where they are implemented
   - Small, focused interfaces (1-3 methods)
   - Existing interfaces (e.g., `Checker`) implemented correctly
   - No interface pollution (don't create interfaces for single implementations)

3. **Resource Cleanup**
   - `defer` for cleanup of resources (files, connections, locks)
   - `defer` placed immediately after resource acquisition
   - No `defer` in loops (resource leak)

4. **Concurrency**
   - No data races (shared state protected by mutex or channels)
   - No goroutine leaks (context cancellation, `sync.WaitGroup`)
   - Channel operations won't deadlock

5. **Naming**
   - Exported names are clear without package prefix
   - Acronyms consistently cased (`URL`, `HTTP`, not `Url`, `Http`)
   - Receiver names short and consistent
   - Test names describe behavior: `TestReconcile_WhenDeleted_RemovesFinalizer`

6. **Package Organization**
   - No circular dependencies
   - Internal types unexported
   - Related functionality grouped logically

### Step 10: Controller-Runtime Specifics

1. **Client Usage**
   - `client.Get()` / `client.List()` for reads
   - `client.Create()` / `client.Update()` / `client.Patch()` for writes
   - `client.Status().Update()` for status subresource
   - Proper use of `client.ObjectKey` / `types.NamespacedName`

2. **RBAC Markers**
   - `// +kubebuilder:rbac` markers match actual API calls in code
   - No over-broad permissions (`*` verbs or resources)
   - New resource types require new RBAC markers
   - Run `make manifests` to regenerate if markers changed

3. **Logging**
   - Structured logging with `logr.Logger` (not `fmt.Printf` or `log.Printf`)
   - Key-value pairs for context: `log.Info("message", "key", value)`
   - Appropriate log levels (Info for normal flow, Error for failures)
   - No sensitive data in log messages

---

## Phase 7: Security Review

1. **RBAC Scope**
   - Principle of least privilege followed
   - No wildcard verbs or resources unless justified
   - ClusterRole vs Role used appropriately

2. **Secret Management**
   - No hardcoded credentials, tokens, or keys
   - Secrets read from Kubernetes Secrets, not ConfigMaps
   - Secret data not logged or included in events
   - TLS certificates handled correctly (proper rotation, CA trust)

3. **Container Security**
   - `runAsNonRoot: true` in security contexts
   - No `privileged: true`
   - `allowPrivilegeEscalation: false`
   - Capabilities dropped where possible

4. **Input Validation**
   - User-provided config bundle validated before use
   - No YAML/JSON injection via user-controlled values
   - Resource names sanitized (DNS-compatible)

---

## Phase 8: OLM Bundle Review (if `bundle/` modified)

1. **ClusterServiceVersion**
   - Version bumped appropriately
   - `replaces` field points to correct previous version
   - RBAC permissions match controller RBAC markers
   - Owned CRDs listed with correct versions

2. **CRD Schema**
   - OpenAPI v3 schema matches Go types
   - Description fields populated
   - Backward compatible with previous versions

---

## Phase 9: Testing Review

1. **Unit Test Coverage**
   - New reconciliation logic has unit tests
   - Table-driven tests for functions with multiple cases
   - Both success and error paths tested
   - Edge cases covered (nil inputs, empty lists, not-found resources)

2. **Test Quality**
   - Tests use `envtest` or fake client correctly
   - No test pollution (shared state between tests)
   - Assertions are specific (not just `err == nil`)
   - Subtests used for related cases: `t.Run("case", func(t *testing.T) {...})`

3. **E2E Test Coverage**
   - Component behavior changes have corresponding e2e tests in `e2e/`
   - kuttl assertions verify expected Kubernetes state

4. **Generated Code**
   - If types changed: `make generate` output included
   - If RBAC changed: `make manifests` output included

---

## Phase 10: Generate Review Report

### Output Format

```
+==============================================================================+
|                     KUBERNETES OPERATOR PR REVIEW                            |
+==============================================================================+
|  PR:             #[number] - [title]                                         |
|  Author:         [author]                                                    |
|  Files Changed:  [count]                                                     |
|  Additions:      +[lines]  Deletions: -[lines]                              |
+==============================================================================+
|                         CHANGE SUMMARY                                       |
+------------------------------------------------------------------------------+
|  [Brief description of what this PR does]                                    |
|                                                                              |
|  PROJQUAY Reference: [PROJQUAY-#### or MISSING]                              |
|                                                                              |
|  Files by Category:                                                          |
|  * CRD/API Types:    [count] files                                           |
|  * Controllers:      [count] files                                           |
|  * Kustomize Code:   [count] files                                           |
|  * YAML Manifests:   [count] files                                           |
|  * Component Status: [count] files                                           |
|  * OLM Bundle:       [count] files                                           |
|  * Tests:            [count] files                                           |
|  * Other:            [count] files                                           |
|                                                                              |
+==============================================================================+
|                    CRD / API COMPATIBILITY                                    |
+------------------------------------------------------------------------------+
|  [If no API changes: "No CRD/API changes in this PR"]                        |
|                                                                              |
|  Backward Compatible:    [YES / NO - details]                                |
|  Field Changes:          [Added/Removed/Modified - list]                     |
|  Validation Changes:     [YES/NO - details]                                  |
|  DeepCopy Regenerated:   [YES/NO/N/A]                                        |
|                                                                              |
+==============================================================================+
|                    RECONCILIATION ANALYSIS                                    |
+------------------------------------------------------------------------------+
|  [If no reconciler changes: "No reconciliation logic changes in this PR"]    |
|                                                                              |
|  Idempotency:            [OK / CONCERN] - [details]                          |
|  Requeue Patterns:       [OK / CONCERN] - [details]                          |
|  Error Handling:         [OK / CONCERN] - [details]                          |
|  Status Updates:         [OK / CONCERN] - [details]                          |
|  Finalizer Handling:     [OK / CONCERN] - [details]                          |
|  Blocking Operations:    [NONE / FOUND] - [details]                          |
|  Context Propagation:    [OK / CONCERN] - [details]                          |
|                                                                              |
+==============================================================================+
|                    KUSTOMIZE / MANIFEST ANALYSIS                              |
+------------------------------------------------------------------------------+
|  [If no manifest changes: "No Kustomize/manifest changes in this PR"]        |
|                                                                              |
|  Resource Specs:         [OK / CONCERN] - [details]                          |
|  Security Contexts:      [OK / CONCERN] - [details]                          |
|  RBAC Changes:           [OK / CONCERN] - [details]                          |
|  Secret Handling:        [OK / CONCERN] - [details]                          |
|                                                                              |
+==============================================================================+
|                    GO CODE QUALITY                                            |
+------------------------------------------------------------------------------+
|                                                                              |
|  * Error Handling:       [Excellent/Good/Acceptable/Needs Work/Poor]         |
|  * Interface Design:     [Excellent/Good/Acceptable/Needs Work/Poor]         |
|  * Idiomatic Go:         [Excellent/Good/Acceptable/Needs Work/Poor]         |
|  * Naming/Organization:  [Excellent/Good/Acceptable/Needs Work/Poor]         |
|  * Logging:              [Excellent/Good/Acceptable/Needs Work/Poor]         |
|  * controller-runtime:   [Excellent/Good/Acceptable/Needs Work/Poor]         |
|                                                                              |
|  Highlights:                                                                 |
|  + [Positive observation 1]                                                  |
|  + [Positive observation 2]                                                  |
|                                                                              |
|  Concerns:                                                                   |
|  ! [Concern 1]                                                               |
|  ! [Concern 2]                                                               |
|                                                                              |
+==============================================================================+
|                    SECURITY ASSESSMENT                                        |
+------------------------------------------------------------------------------+
|  [Any security considerations - or "No security concerns identified"]        |
|                                                                              |
|  * RBAC Scope:           [OK / CONCERN] - [details]                          |
|  * Secret Management:    [OK / CONCERN] - [details]                          |
|  * Container Security:   [OK / CONCERN] - [details]                          |
|  * Input Validation:     [OK / CONCERN] - [details]                          |
|                                                                              |
+==============================================================================+
|                    OLM BUNDLE ASSESSMENT                                      |
+------------------------------------------------------------------------------+
|  [If no bundle changes: "No OLM bundle changes in this PR"]                  |
|                                                                              |
|  * CSV Version:          [OK / CONCERN] - [details]                          |
|  * RBAC Consistency:     [OK / CONCERN] - [details]                          |
|  * CRD Schema:           [OK / CONCERN] - [details]                          |
|                                                                              |
+==============================================================================+
|                    TESTING ASSESSMENT                                         |
+------------------------------------------------------------------------------+
|  Test Coverage:          [Excellent/Good/Acceptable/Needs Work/None]         |
|  Test Quality:           [Excellent/Good/Acceptable/Needs Work/None]         |
|                                                                              |
|  [x/o] Unit tests for new logic                                              |
|  [x/o] Error paths tested                                                    |
|  [x/o] Table-driven tests where appropriate                                  |
|  [x/o] E2E tests for component behavior changes                             |
|  [x/o] Generated code updated (make generate / make manifests)               |
|                                                                              |
+==============================================================================+
|                    CRITICAL ISSUES                                            |
+------------------------------------------------------------------------------+
|  [Issues that MUST be fixed before merge]                                    |
|                                                                              |
|  1. [Issue description]                                                      |
|     Location: [file:line]                                                    |
|     Problem:  [What's wrong]                                                 |
|     Fix:      [How to fix it]                                                |
|                                                                              |
+==============================================================================+
|                    WARNINGS                                                   |
+------------------------------------------------------------------------------+
|  [Non-blocking concerns that should be considered]                           |
|                                                                              |
|  1. [Warning description]                                                    |
|     Location: [file:line]                                                    |
|     Suggestion: [Improvement idea]                                           |
|                                                                              |
+==============================================================================+
|                    RECOMMENDATIONS                                            |
+------------------------------------------------------------------------------+
|  * [Recommendation 1]                                                        |
|  * [Recommendation 2]                                                        |
|                                                                              |
+==============================================================================+
|                           VERDICT                                            |
+------------------------------------------------------------------------------+
|                                                                              |
|  [APPROVE / APPROVE WITH COMMENTS / REQUEST CHANGES / BLOCK]                |
|                                                                              |
|  Summary: [1-2 sentence overall assessment]                                  |
|                                                                              |
|  Key Takeaways:                                                              |
|  * [Main point 1]                                                            |
|  * [Main point 2]                                                            |
|  * [Main point 3]                                                            |
|                                                                              |
+==============================================================================+
```

---

## Review Guidelines

### Approve when:
- No critical issues found
- Code follows idiomatic Go and controller-runtime patterns
- Reconciliation is idempotent with correct requeue behavior
- CRD changes are backward compatible
- RBAC follows least privilege
- Tests adequately cover new logic
- Commit messages reference PROJQUAY Jira ticket

### Approve with Comments when:
- Minor Go style improvements possible but not blocking
- Additional test coverage recommended but not required
- Minor naming or organization suggestions

### Request Changes when:
- Non-idempotent reconciliation logic
- Missing error handling or swallowed errors
- Incorrect requeue patterns (blocking operations, time.Sleep in Reconcile)
- RBAC over-scoped without justification
- Missing tests for new reconciliation paths
- Status updates not using status subresource
- Generated code not regenerated after type changes
- Missing PROJQUAY Jira reference in commits

### Block when:
- CRD breaking changes (field removal, type change) without migration path
- Security vulnerabilities (privilege escalation, secret leaks, hardcoded credentials)
- Blocking operations in Reconcile loop (time.Sleep, synchronous polling)
- Fundamentally broken reconciliation (infinite requeue, no error handling)
- Finalizer logic that could orphan external resources or deadlock deletion
- Owner reference or RBAC changes that could cause cascading failures

---

## Example Usage

```
/review-pr 1150
/review-pr https://github.com/quay/quay-operator/pull/1150
```

This will:
1. Fetch the PR details and diff
2. Validate commit message format and PROJQUAY reference
3. Categorize all changed files by area
4. Analyze CRD/API changes for backward compatibility
5. Review reconciliation logic for correctness and idempotency
6. Check Kustomize manifests for security and best practices
7. Evaluate Go code quality against senior engineer standards
8. Assess RBAC, security, and OLM bundle changes
9. Review test coverage and quality
10. Generate a comprehensive review report with verdict
