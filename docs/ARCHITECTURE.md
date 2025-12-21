# Architecture

## Overview

`zen-gc` is a Kubernetes controller that implements generic garbage collection policies for any Kubernetes resource. It follows the standard Kubernetes controller pattern with informers, work queues, and reconciliation loops.

## System Architecture

```mermaid
graph TB
    subgraph "Kubernetes Cluster"
        API[Kubernetes API Server]
        CRD[GarbageCollectionPolicy CRD]
        Resources[Target Resources<br/>Pods, ConfigMaps, Jobs, etc.]
    end
    
    subgraph "GC Controller"
        Main[Main Controller]
        PolicyInformer[Policy Informer]
        ResourceInformer[Resource Informers<br/>Dynamic]
        Queue[Work Queue]
        Reconciler[Reconciler]
        RateLimiter[Rate Limiter]
        Metrics[Metrics Exporter]
    end
    
    subgraph "Components"
        StatusUpdater[Status Updater]
        EventRecorder[Event Recorder]
        Validator[Policy Validator]
    end
    
    API -->|Watch| PolicyInformer
    API -->|Watch| ResourceInformer
    PolicyInformer -->|Events| Queue
    ResourceInformer -->|Cache| Reconciler
    Queue -->|Reconcile| Reconciler
    Reconciler -->|Update| StatusUpdater
    Reconciler -->|Record| EventRecorder
    Reconciler -->|Validate| Validator
    Reconciler -->|Rate Limit| RateLimiter
    Reconciler -->|Delete| Resources
    Reconciler -->|Metrics| Metrics
    StatusUpdater -->|Update Status| CRD
    EventRecorder -->|Create Events| API
    Metrics -->|Expose| Prometheus
```

## Component Details

### 1. Main Controller (`cmd/gc-controller/main.go`)

The entry point that:
- Initializes Kubernetes clients (dynamic, core)
- Sets up leader election for HA
- Creates and starts the GC controller
- Configures metrics server
- Handles graceful shutdown

```mermaid
sequenceDiagram
    participant Main
    participant LeaderElection
    participant GCController
    participant MetricsServer
    
    Main->>LeaderElection: Start Leader Election
    LeaderElection->>GCController: Start (if leader)
    GCController->>GCController: Initialize Informers
    GCController->>GCController: Start Workers
    GCController->>MetricsServer: Register Metrics
    MetricsServer->>MetricsServer: Start HTTP Server
    Note over GCController: Process Policies
    Main->>GCController: Stop (on signal)
    GCController->>GCController: Graceful Shutdown
```

### 2. GC Controller (`pkg/controller/gc_controller.go`)

Core controller logic:

**Responsibilities:**
- Watch `GarbageCollectionPolicy` CRDs
- Create dynamic informers for target resources
- Evaluate policies against resources
- Delete resources that match TTL/conditions
- Update policy status
- Emit metrics and events

**Key Methods:**
- `NewGCController()`: Initialize controller with clients
- `Start()`: Start informers and workers
- `evaluatePolicies()`: Main reconciliation loop
- `evaluatePolicy()`: Evaluate single policy
- `deleteResource()`: Delete resource with rate limiting

```mermaid
flowchart TD
    Start[Start Controller] --> Init[Initialize Informers]
    Init --> Watch[Watch Policies]
    Watch --> Event{Policy Event}
    Event -->|Add/Update| CreateInformer[Create Resource Informer]
    Event -->|Delete| RemoveInformer[Remove Resource Informer]
    CreateInformer --> Reconcile[Reconcile Loop]
    RemoveInformer --> Reconcile
    Reconcile --> GetPolicies[Get All Policies]
    GetPolicies --> ForEach{For Each Policy}
    ForEach -->|Active| Evaluate[Evaluate Policy]
    ForEach -->|Paused| Skip[Skip Policy]
    Evaluate --> GetResources[Get Resources from Cache]
    GetResources --> Filter[Filter by Selectors]
    Filter --> CheckTTL[Check TTL]
    CheckTTL -->|Expired| CheckConditions[Check Conditions]
    CheckTTL -->|Not Expired| Next[Next Resource]
    CheckConditions -->|Met| Delete[Delete Resource]
    CheckConditions -->|Not Met| Next
    Delete --> RateLimit[Rate Limiter]
    RateLimit --> DeleteAPI[Delete via API]
    DeleteAPI --> UpdateStatus[Update Policy Status]
    UpdateStatus --> EmitMetrics[Emit Metrics]
    EmitMetrics --> Next
    Next --> More{More Resources?}
    More -->|Yes| GetResources
    More -->|No| MorePolicies{More Policies?}
    MorePolicies -->|Yes| ForEach
    MorePolicies -->|No| Wait[Wait Interval]
    Wait --> Reconcile
```

### 3. Policy Evaluation Flow

```mermaid
flowchart LR
    Policy[GarbageCollectionPolicy] --> Target[Target Resource Spec]
    Target --> Selectors[Label/Field Selectors]
    Target --> Namespace[Namespace Scope]
    
    Resources[Kubernetes Resources] --> Match{Match Selectors?}
    Selectors --> Match
    Namespace --> Match
    
    Match -->|Yes| TTL[Calculate TTL]
    Match -->|No| Skip[Skip Resource]
    
    TTL --> Expired{TTL Expired?}
    Expired -->|No| Skip
    Expired -->|Yes| Conditions[Check Conditions]
    
    Conditions --> Phase{Phase Match?}
    Conditions --> Labels{Labels Match?}
    Conditions --> Annotations{Annotations Match?}
    Conditions --> Fields{Field Conditions?}
    
    Phase -->|All Pass| Delete[Delete Resource]
    Labels -->|All Pass| Delete
    Annotations -->|All Pass| Delete
    Fields -->|All Pass| Delete
    
    Phase -->|Fail| Skip
    Labels -->|Fail| Skip
    Annotations -->|Fail| Skip
    Fields -->|Fail| Skip
    
    Delete --> RateLimit[Rate Limiter]
    RateLimit --> Behavior[Apply Behavior]
    Behavior --> DryRun{Dry Run?}
    DryRun -->|Yes| Log[Log Only]
    DryRun -->|No| DeleteAPI[Delete via API]
    DeleteAPI --> Metrics[Record Metrics]
    Metrics --> Event[Create Event]
```

### 4. Informer Architecture

```mermaid
graph TB
    subgraph "Policy Informer"
        PolicyGVR[Policy GVR<br/>gc.kube-zen.io/v1alpha1]
        PolicyInformer[Shared Informer]
        PolicyStore[Policy Store]
    end
    
    subgraph "Resource Informers"
        ResourceGVR1[Resource GVR 1<br/>v1/ConfigMap]
        ResourceGVR2[Resource GVR 2<br/>apps/v1/Deployment]
        ResourceGVRN[Resource GVR N<br/>...]
        
        ResourceInformer1[Resource Informer 1]
        ResourceInformer2[Resource Informer 2]
        ResourceInformerN[Resource Informer N]
        
        ResourceStore1[Resource Store 1]
        ResourceStore2[Resource Store 2]
        ResourceStoreN[Resource Store N]
    end
    
    PolicyGVR --> PolicyInformer
    PolicyInformer --> PolicyStore
    
    ResourceGVR1 --> ResourceInformer1
    ResourceGVR2 --> ResourceInformer2
    ResourceGVRN --> ResourceInformerN
    
    ResourceInformer1 --> ResourceStore1
    ResourceInformer2 --> ResourceStore2
    ResourceInformerN --> ResourceStoreN
    
    PolicyStore -->|Policies| Controller[GC Controller]
    ResourceStore1 -->|Resources| Controller
    ResourceStore2 -->|Resources| Controller
    ResourceStoreN -->|Resources| Controller
```

### 5. Rate Limiting

```mermaid
sequenceDiagram
    participant Reconciler
    participant RateLimiter
    participant TokenBucket
    participant API
    
    Reconciler->>RateLimiter: Wait() for deletion
    RateLimiter->>TokenBucket: Request Token
    TokenBucket->>TokenBucket: Check Available Tokens
    alt Tokens Available
        TokenBucket->>Reconciler: Token Granted
        Reconciler->>API: Delete Resource
        API-->>Reconciler: Success
    else No Tokens
        TokenBucket->>Reconciler: Wait
        Note over TokenBucket: Refill Tokens
        TokenBucket->>Reconciler: Token Granted
        Reconciler->>API: Delete Resource
    end
```

### 6. Metrics Flow

```mermaid
graph LR
    Controller[GC Controller] --> Metrics[Metrics Package]
    Metrics --> Gauge[Gauges<br/>Policies by Phase]
    Metrics --> Counter[Counters<br/>Deletions, Errors]
    Metrics --> Histogram[Histograms<br/>Durations]
    
    Gauge --> Prometheus[Prometheus]
    Counter --> Prometheus
    Histogram --> Prometheus
    
    Prometheus --> Grafana[Grafana Dashboards]
    Prometheus --> Alerts[PrometheusRules]
```

## Data Flow

### Policy Creation Flow

```mermaid
sequenceDiagram
    participant User
    participant API
    participant CRD
    participant Controller
    participant Informer
    
    User->>API: Create GarbageCollectionPolicy
    API->>CRD: Store Policy
    CRD->>Informer: Policy Added Event
    Informer->>Controller: Policy Event
    Controller->>Controller: Create Resource Informer
    Controller->>Informer: Watch Target Resources
    Informer->>Controller: Resource Events
    Controller->>Controller: Evaluate Policy
    Controller->>API: Delete Resources (if needed)
    Controller->>CRD: Update Policy Status
```

### Resource Deletion Flow

```mermaid
sequenceDiagram
    participant Controller
    participant RateLimiter
    participant Validator
    participant API
    participant Resource
    participant StatusUpdater
    participant EventRecorder
    
    Controller->>Validator: Validate Policy
    Validator-->>Controller: Valid
    Controller->>RateLimiter: Wait for Rate Limit
    RateLimiter-->>Controller: Token Granted
    Controller->>API: Delete Resource
    API->>Resource: Delete Request
    Resource-->>API: Deleted
    API-->>Controller: Success
    Controller->>StatusUpdater: Update Status
    Controller->>EventRecorder: Record Event
    Controller->>Controller: Record Metrics
```

## High Availability

```mermaid
graph TB
    subgraph "Leader Election"
        Pod1[GC Controller Pod 1]
        Pod2[GC Controller Pod 2]
        Pod3[GC Controller Pod 3]
        Lease[Lease Resource]
    end
    
    Pod1 -->|Acquire Lease| Lease
    Pod2 -->|Try Acquire| Lease
    Pod3 -->|Try Acquire| Lease
    
    Lease -->|Leader| Pod1
    Lease -.->|Follower| Pod2
    Lease -.->|Follower| Pod3
    
    Pod1 -->|Active| Work[Process Policies]
    Pod2 -.->|Standby| Wait[Wait]
    Pod3 -.->|Standby| Wait
    
    Pod1 -.->|Fail| Lease
    Lease -->|New Leader| Pod2
```

## Security Model

```mermaid
graph TB
    subgraph "RBAC"
        SA[ServiceAccount]
        CR[ClusterRole]
        CRB[ClusterRoleBinding]
    end
    
    subgraph "Permissions"
        Read[Read Policies]
        Watch[Watch Resources]
        Delete[Delete Resources]
        Update[Update Status]
        Events[Create Events]
    end
    
    subgraph "Pod Security"
        NonRoot[Non-Root User]
        ReadOnly[Read-Only Root FS]
        DropAll[Drop All Capabilities]
    end
    
    SA --> CRB
    CRB --> CR
    CR --> Read
    CR --> Watch
    CR --> Delete
    CR --> Update
    CR --> Events
    
    Pod --> NonRoot
    Pod --> ReadOnly
    Pod --> DropAll
```

## Deployment Architecture

```mermaid
graph TB
    subgraph "Kubernetes Cluster"
        subgraph "Namespace: gc-system"
            Deployment[GC Controller Deployment]
            Service[Service<br/>Metrics Port]
            SA[ServiceAccount]
        end
        
        subgraph "Cluster Scope"
            CRD[GarbageCollectionPolicy CRD]
            CR[ClusterRole]
            CRB[ClusterRoleBinding]
        end
        
        subgraph "User Namespaces"
            Policy1[Policy: cleanup-configmaps]
            Policy2[Policy: cleanup-pods]
            PolicyN[Policy: ...]
        end
    end
    
    Deployment --> SA
    SA --> CRB
    CRB --> CR
    CR --> CRD
    CR --> Resources[Target Resources]
    
    Policy1 --> Deployment
    Policy2 --> Deployment
    PolicyN --> Deployment
    
    Deployment --> Service
    Service --> Prometheus[Prometheus]
```

## Performance Considerations

### Informer Caching

- **Policy Informer**: Single informer for all policies (cluster-wide or namespace-scoped)
- **Resource Informers**: One informer per unique GVR (GroupVersionResource)
- **Cache Efficiency**: Resources cached in memory, reducing API server load
- **Resync Period**: Configurable resync interval (default: 1 minute)

### Rate Limiting

- **Token Bucket Algorithm**: Smooth rate limiting with burst support
- **Per-Policy Rate**: Each policy can specify `maxDeletionsPerSecond`
- **Default Rate**: 10 deletions/second (configurable)
- **Batching**: Optional batch size for efficient deletions

### Scalability

- **Horizontal Scaling**: Multiple controller replicas with leader election
- **Resource Limits**: Configurable CPU/memory limits
- **Worker Threads**: Configurable number of worker goroutines
- **Queue Depth**: Work queue prevents memory bloat

## Error Handling

```mermaid
flowchart TD
    Error[Error Occurs] --> Type{Error Type}
    
    Type -->|API Error| Retry{Retry?}
    Type -->|Validation Error| Log[Log Error]
    Type -->|Rate Limit| Wait[Wait & Retry]
    
    Retry -->|Yes| Backoff[Exponential Backoff]
    Retry -->|No| Log
    
    Backoff --> RetryAPI[Retry API Call]
    RetryAPI --> Success{Success?}
    Success -->|Yes| Continue[Continue]
    Success -->|No| MaxRetries{Max Retries?}
    
    MaxRetries -->|No| Backoff
    MaxRetries -->|Yes| RecordError[Record Error Metric]
    RecordError --> Event[Create Error Event]
    Event --> Log
    
    Log --> Metrics[Update Error Metrics]
    Metrics --> Continue
```

## Monitoring & Observability

### Metrics

- **Policy Metrics**: Number of policies by phase
- **Resource Metrics**: Matched, deleted, pending resources
- **Performance Metrics**: Evaluation duration, deletion duration
- **Error Metrics**: Error counts by type

### Events

- **Policy Events**: Policy evaluation started/completed/failed
- **Resource Events**: Resource deleted with reason
- **Error Events**: Deletion failures, status update failures

### Logging

- **Structured Logging**: Using klog with structured fields
- **Log Levels**: Configurable verbosity (V levels)
- **Context**: Policy name, resource name, namespace in logs

## Extension Points

### Custom TTL Calculations

The controller supports multiple TTL calculation methods:
- Fixed TTL (`secondsAfterCreation`)
- Field-based TTL (`fieldPath`)
- Relative TTL (`relativeTo`)
- Mapped TTL (`mappings`)

### Custom Conditions

Policies can specify complex conditions:
- Phase matching
- Label matching
- Annotation matching
- Field conditions (equals, not equals, in, not in, etc.)

### Behavior Customization

Each policy can customize deletion behavior:
- Rate limiting (`maxDeletionsPerSecond`)
- Batch size (`batchSize`)
- Dry run mode (`dryRun`)
- Grace period (`gracePeriodSeconds`)
- Propagation policy (`propagationPolicy`)

