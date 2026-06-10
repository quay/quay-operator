# Architecture

The quay-operator is a Kubernetes operator that deploys and manages [Quay container registry](https://github.com/quay/quay) instances. It watches `QuayRegistry` custom resources and reconciles them into the full set of Kubernetes objects needed to run Quay and its dependencies.

## System Overview

```mermaid
graph TB
    User["User / GitOps"]
    QR["QuayRegistry CR"]
    MainR["Main Reconciler"]
    StatusR["Status Reconciler"]
    Kustomize["Kustomize Inflation"]
    Middleware["Middleware Processing"]
    K8s["Kubernetes API"]

    User -->|"create/update"| QR
    QR -->|"watch"| MainR
    QR -->|"watch (1min poll)"| StatusR

    MainR -->|"4. gather context"| ClusterAPIs["Cluster APIs<br/>(Route, OBC, Monitoring)"]
    MainR -->|"6. inflate"| Kustomize
    Kustomize -->|"[]client.Object"| Middleware
    Middleware -->|"7. apply"| K8s

    StatusR -->|"evaluate"| CmpStatus["Component Status<br/>Checkers"]
    CmpStatus -->|"read"| K8s
    StatusR -->|"update conditions"| QR

    K8s -->|"creates"| Resources["Deployments<br/>Services<br/>Secrets<br/>ConfigMaps<br/>Routes<br/>HPAs<br/>Jobs"]
```

## Reconciliation Flow

The main reconciler processes each `QuayRegistry` through an 8-step sequence. Each step can short-circuit with a requeue or error condition.

```mermaid
flowchart TD
    Start["Reconcile triggered"] --> Deleted{Flagged for<br/>deletion?}
    Deleted -->|yes| Finalize["Run finalizer<br/>(cleanup Grafana, labels)"]
    Deleted -->|no| Migration{Migration or<br/>upgrade running?}
    Migration -->|yes| WaitJob["Check job status,<br/>requeue"]
    Migration -->|no| Bundle{Config bundle<br/>exists?}
    Bundle -->|no| CreateBundle["Create initial<br/>config bundle"]
    Bundle -->|yes| Context["Gather context<br/>(TLS, routes, storage,<br/>monitoring, databases)"]
    Context --> Validate["Validate components<br/>& overrides"]
    Validate --> Inflate["Kustomize inflation<br/>(base + overlays)"]
    Inflate --> Apply["Apply objects<br/>(middleware + SSA)"]
    Apply --> Status["Update status<br/>(endpoint, conditions)"]
```

## Component Architecture

The operator manages 11 component types. Each can be independently managed or unmanaged.

```mermaid
graph LR
    subgraph Required
        PG["postgres"]
        Redis["redis"]
        ObjStore["objectstorage"]
        Route["route"]
        TLS["tls"]
    end

    subgraph Optional
        Clair["clair"]
        ClairPG["clairpostgres"]
        HPA["horizontalpodautoscaler"]
        Mirror["mirror"]
        Mon["monitoring"]
    end

    subgraph Always Managed
        Quay["quay"]
    end

    Quay -->|depends on| PG
    Quay -->|depends on| Redis
    Quay -->|depends on| ObjStore
    Quay -->|depends on| TLS
    Clair -->|depends on| ClairPG
    Mirror -->|depends on| Quay
    HPA -->|scales| Quay
    HPA -->|scales| Clair
    HPA -->|scales| Mirror
```

## Status Evaluation

Component health is evaluated in dependency order. If any Quay dependency is unhealthy, Quay and Mirror are marked as not ready without being checked.

```mermaid
flowchart TD
    subgraph "Phase 1: Independent"
        HPA_C["HPA"]
        Route_C["Route"]
        Mon_C["Monitoring"]
    end

    subgraph "Phase 2: Quay Dependencies"
        PG_C["Postgres"]
        ObjS_C["ObjectStorage"]
        Clair_C["Clair"]
        ClairPG_C["ClairPostgres"]
        TLS_C["TLS"]
        Redis_C["Redis"]
    end

    subgraph "Phase 3: Dependent (only if Phase 2 passes)"
        Quay_C["Quay"]
        Mirror_C["Mirror"]
    end

    PG_C & ObjS_C & Clair_C & ClairPG_C & TLS_C & Redis_C -->|"all healthy?"| Check{"All deps<br/>healthy?"}
    Check -->|yes| Quay_C & Mirror_C
    Check -->|"no"| Skip["Quay + Mirror marked<br/>'Awaiting component X'"]
```

## Key Design Decisions

**Two reconcilers instead of one.** The main reconciler runs database migrations when the Quay version changes. If status evaluation shared the same requeue loop, migrations would re-run on every 1-minute tick. Separating them allows frequent status polling without expensive side effects.

**Kustomize for manifest generation.** Component manifests are stored as standard Kustomize overlays rather than Go templates. This makes the manifests readable, testable with standard tools, and allows operators to inspect what will be applied before the operator transforms it.

**Server-side apply with ForceOwnership.** The operator is the sole owner of managed resource fields. This is intentional — it prevents configuration drift where manual patches to managed resources survive across reconciles. Users customize via the `QuayRegistry` spec (overrides, config bundle), not by patching managed resources directly.

**Middleware for what Kustomize can't do.** Some transforms (injecting user-specified env vars, applying resource overrides, stripping resource requests for dev mode) require Go logic that Kustomize's patch system cannot express. The middleware layer runs after inflation and before cluster apply.
