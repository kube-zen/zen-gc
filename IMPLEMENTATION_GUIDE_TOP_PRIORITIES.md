# Implementation Guide: Top 5 Priorities for Production-Grade KEP

This guide provides step-by-step implementation for the 5 highest-priority items to make zen-gc production-grade.

---

## Priority 1: Leader Election for HA (CRITICAL)

### Why
- Enables multi-replica deployments
- Zero-downtime upgrades
- Production HA requirement

### Implementation Steps

#### Step 1: Add Dependencies
```bash
go get sigs.k8s.io/controller-runtime/pkg/manager
go get sigs.k8s.io/controller-runtime/pkg/leaderelection
```

#### Step 2: Update main.go
```go
package main

import (
    "context"
    "flag"
    "os"
    "os/signal"
    "syscall"
    
    "sigs.k8s.io/controller-runtime/pkg/client/config"
    "sigs.k8s.io/controller-runtime/pkg/manager"
    "sigs.k8s.io/controller-runtime/pkg/manager/signals"
    
    "github.com/kube-zen/zen-gc/pkg/controller"
)

var (
    metricsAddr     = flag.String("metrics-addr", ":8080", "Metrics address")
    enableLeaderElection = flag.Bool("enable-leader-election", true, "Enable leader election")
    leaderElectionNamespace = flag.String("leader-election-namespace", "", "Leader election namespace")
)

func main() {
    flag.Parse()
    
    cfg, err := config.GetConfig()
    if err != nil {
        klog.Fatalf("Error getting config: %v", err)
    }
    
    // Create manager with leader election
    mgr, err := manager.New(cfg, manager.Options{
        MetricsBindAddress:     *metricsAddr,
        LeaderElection:          *enableLeaderElection,
        LeaderElectionNamespace: *leaderElectionNamespace,
        LeaderElectionID:        "gc-controller-leader-election",
    })
    if err != nil {
        klog.Fatalf("Error creating manager: %v", err)
    }
    
    // Create GC controller
    gcController, err := controller.NewGCController(mgr.GetDynamicClient())
    if err != nil {
        klog.Fatalf("Error creating GC controller: %v", err)
    }
    
    // Start controller
    if err := gcController.Start(); err != nil {
        klog.Fatalf("Error starting GC controller: %v", err)
    }
    
    // Start manager (handles leader election)
    if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
        klog.Fatalf("Error starting manager: %v", err)
    }
}
```

#### Step 3: Update Deployment
```yaml
# deploy/manifests/deployment.yaml
spec:
  replicas: 2  # Enable HA
  template:
    spec:
      containers:
        - name: gc-controller
          args:
            - --enable-leader-election=true
            - --leader-election-namespace=gc-system
```

**Estimated Time**: 4-6 hours  
**Impact**: HIGH - Enables production HA

---

## Priority 2: Proper Status Updates (CRITICAL)

### Why
- Users need visibility into policy status
- Critical for debugging
- Standard Kubernetes pattern

### Implementation Steps

#### Step 1: Add Typed Client
```go
// pkg/controller/gc_controller.go
import (
    "sigs.k8s.io/controller-runtime/pkg/client"
    gcapi "github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

type GCController struct {
    dynamicClient dynamic.Interface
    client        client.Client  // Add typed client
    // ... existing fields
}

func NewGCController(dynamicClient dynamic.Interface, client client.Client) (*GCController, error) {
    return &GCController{
        dynamicClient: dynamicClient,
        client:        client,
        // ... rest of initialization
    }, nil
}
```

#### Step 2: Implement Status Update
```go
// pkg/controller/gc_controller.go
func (gc *GCController) updatePolicyStatus(
    ctx context.Context,
    policy *v1alpha1.GarbageCollectionPolicy,
    matched, deleted, pending int64,
) error {
    // Create patch
    patch := client.MergeFrom(policy.DeepCopy())
    
    // Update status
    now := metav1.Now()
    policy.Status.ResourcesMatched = matched
    policy.Status.ResourcesDeleted = deleted
    policy.Status.ResourcesPending = pending
    policy.Status.LastGCRun = &now
    
    // Calculate next run
    nextRun := now.Add(DefaultGCInterval)
    policy.Status.NextGCRun = &metav1.Time{Time: nextRun}
    
    // Update phase
    if policy.Status.Phase == "" {
        policy.Status.Phase = "Active"
    }
    
    // Patch status
    return gc.client.Status().Patch(ctx, policy, patch)
}
```

#### Step 3: Update Calls
```go
// In evaluatePolicy
if err := gc.updatePolicyStatus(gc.ctx, policy, matchedCount, deletedCount, pendingCount); err != nil {
    recordError(policy.Namespace, policy.Name, "status_update_failed")
    return fmt.Errorf("failed to update status: %w", err)
}
```

**Estimated Time**: 3-4 hours  
**Impact**: HIGH - Critical user visibility

---

## Priority 3: Kubernetes Events (HIGH)

### Why
- Standard Kubernetes observability
- Integrates with kubectl describe
- Better debugging

### Implementation Steps

#### Step 1: Add Event Recorder
```go
// pkg/controller/gc_controller.go
import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/record"
)

type GCController struct {
    // ... existing fields
    eventRecorder record.EventRecorder
}

func NewGCController(
    dynamicClient dynamic.Interface,
    client client.Client,
    eventRecorder record.EventRecorder,
) (*GCController, error) {
    return &GCController{
        // ... existing fields
        eventRecorder: eventRecorder,
    }, nil
}
```

#### Step 2: Emit Events
```go
// In evaluatePolicy
gc.eventRecorder.Eventf(
    policy,
    "Normal",
    "PolicyEvaluated",
    "Evaluated policy: matched=%d, deleted=%d, pending=%d",
    matchedCount, deletedCount, pendingCount,
)

// In deleteResource
gc.eventRecorder.Eventf(
    policy,
    "Normal",
    "ResourceDeleted",
    "Deleted resource %s/%s (reason: %s)",
    resource.GetNamespace(), resource.GetName(), reason,
)

// On errors
gc.eventRecorder.Eventf(
    policy,
    "Warning",
    "EvaluationFailed",
    "Failed to evaluate policy: %v",
    err,
)
```

#### Step 3: Setup in main.go
```go
// Create event broadcaster
eventBroadcaster := record.NewBroadcaster()
eventBroadcaster.StartStructuredLogging(0)
eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{
    Interface: kubeClient.CoreV1().Events(""),
})

eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{
    Component: "gc-controller",
})
```

**Estimated Time**: 3-4 hours  
**Impact**: HIGH - Better observability

---

## Priority 4: Exponential Backoff (HIGH)

### Why
- Prevents API server overload
- Production resilience
- Handles transient errors

### Implementation Steps

#### Step 1: Add Backoff Helper
```go
// pkg/controller/backoff.go
package controller

import (
    "time"
    "k8s.io/apimachinery/pkg/util/wait"
)

var (
    defaultBackoff = wait.Backoff{
        Steps:    5,
        Duration: 100 * time.Millisecond,
        Factor:   2.0,
        Jitter:   0.1,
        Cap:      30 * time.Second,
    }
)

func (gc *GCController) deleteResourceWithBackoff(
    ctx context.Context,
    resource *unstructured.Unstructured,
    policy *v1alpha1.GarbageCollectionPolicy,
) error {
    var lastErr error
    
    err := wait.ExponentialBackoff(defaultBackoff, func() (bool, error) {
        err := gc.deleteResource(resource, policy)
        if err != nil {
            // Check if error is retryable
            if errors.IsTimeout(err) || errors.IsServerTimeout(err) || 
               errors.IsTooManyRequests(err) {
                lastErr = err
                return false, nil // retry
            }
            return false, err // don't retry
        }
        return true, nil // success
    })
    
    if err == wait.ErrWaitTimeout {
        return fmt.Errorf("deletion failed after retries: %w", lastErr)
    }
    
    return err
}
```

#### Step 2: Update deleteResource Calls
```go
// In evaluatePolicy, replace deleteResource with deleteResourceWithBackoff
if err := gc.deleteResourceWithBackoff(gc.ctx, resource, policy); err != nil {
    // ... error handling
}
```

**Estimated Time**: 2-3 hours  
**Impact**: HIGH - Production resilience

---

## Priority 5: Structured Logging (HIGH)

### Why
- Enterprise logging requirements
- Better log aggregation
- Compliance needs

### Implementation Steps

#### Step 1: Add Zap Logger
```bash
go get go.uber.org/zap
go get go.uber.org/zap/zapcore
```

#### Step 2: Create Logger Helper
```go
// pkg/controller/logger.go
package controller

import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func initLogger() {
    config := zap.NewProductionConfig()
    config.EncoderConfig.TimeKey = "timestamp"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    
    var err error
    logger, err = config.Build()
    if err != nil {
        panic(err)
    }
}

func getLogger() *zap.Logger {
    if logger == nil {
        initLogger()
    }
    return logger
}
```

#### Step 3: Replace klog Calls
```go
// Replace klog.Info with structured logging
getLogger().Info("evaluating policy",
    zap.String("policy", policy.Name),
    zap.String("namespace", policy.Namespace),
    zap.String("phase", policy.Status.Phase),
)

getLogger().Info("resource deleted",
    zap.String("policy", policy.Name),
    zap.String("resource_namespace", resource.GetNamespace()),
    zap.String("resource_name", resource.GetName()),
    zap.String("reason", reason),
    zap.Duration("duration", time.Since(deleteStart)),
)

getLogger().Error("evaluation failed",
    zap.String("policy", policy.Name),
    zap.String("namespace", policy.Namespace),
    zap.Error(err),
)
```

**Estimated Time**: 4-5 hours  
**Impact**: MEDIUM-HIGH - Enterprise requirement

---

## Quick Implementation Checklist

### Week 1: Critical Foundation
- [ ] Day 1-2: Leader Election (#1)
- [ ] Day 2-3: Status Updates (#2)
- [ ] Day 3-4: Kubernetes Events (#3)
- [ ] Day 4-5: Exponential Backoff (#4)

### Week 2: Polish & Testing
- [ ] Day 1-2: Structured Logging (#5)
- [ ] Day 2-3: Integration tests for new features
- [ ] Day 3-4: Documentation updates
- [ ] Day 4-5: E2E validation

---

## Expected Outcomes

### After Week 1:
- ✅ HA support (multi-replica)
- ✅ User visibility (status updates)
- ✅ Better observability (events)
- ✅ Production resilience (backoff)

### After Week 2:
- ✅ Enterprise logging
- ✅ Full test coverage
- ✅ Updated documentation

**Status**: Development → **Production-Ready**  
**Production Readiness**: 70% → **90%**

---

## Testing Each Feature

### Leader Election Test
```bash
# Deploy 2 replicas
kubectl scale deployment gc-controller --replicas=2 -n gc-system

# Verify only one leader
kubectl get lease gc-controller-leader-election -n gc-system

# Check logs - only one should be active
kubectl logs -n gc-system -l app=gc-controller
```

### Status Update Test
```bash
# Create policy
kubectl apply -f examples/observation-retention.yaml

# Check status
kubectl get garbagecollectionpolicy observation-retention -o yaml

# Verify status fields populated
```

### Events Test
```bash
# Create policy and wait for evaluation
kubectl describe garbagecollectionpolicy observation-retention

# Should see events in output
```

### Backoff Test
```bash
# Simulate API server errors
# Verify retries in logs
# Check exponential backoff timing
```

---

## Next Steps After Top 5

1. **Admission Webhook** (Week 3)
2. **Integration Tests** (Week 3)
3. **E2E Tests** (Week 4)
4. **Performance Benchmarks** (Week 4)
5. **Helm Chart** (Week 5)

---

## Resources

- [Controller Runtime Leader Election](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/leaderelection)
- [Kubernetes Events](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/)
- [Exponential Backoff](https://pkg.go.dev/k8s.io/apimachinery/pkg/util/wait#Backoff)
- [Zap Logger](https://github.com/uber-go/zap)

