# Disaster Recovery

This document describes disaster recovery procedures for zen-gc, including recovery from accidental mass deletions, backup strategies, emergency stop procedures, and rollback scenarios.

## Table of Contents

- [Recovery from Accidental Mass Deletions](#recovery-from-accidental-mass-deletions)
- [Backup Strategies](#backup-strategies)
- [Emergency Stop Procedures](#emergency-stop-procedures)
- [Rollback Scenarios](#rollback-scenarios)

---

## Recovery from Accidental Mass Deletions

### Immediate Response

If you discover that resources are being deleted unexpectedly:

#### Step 1: Stop the Controller (Emergency)

```bash
# Scale down the controller to zero replicas
kubectl scale deployment gc-controller -n gc-system --replicas=0

# Or delete the deployment entirely
kubectl delete deployment gc-controller -n gc-system
```

#### Step 2: Disable All Policies

```bash
# List all policies
kubectl get garbagecollectionpolicies --all-namespaces

# Annotate policies to pause them (if supported)
kubectl annotate garbagecollectionpolicy <policy-name> -n <namespace> gc.kube-zen.io/paused=true

# Or delete policies
kubectl delete garbagecollectionpolicy <policy-name> -n <namespace>
```

#### Step 3: Assess Damage

```bash
# Check what resources were deleted (from audit logs)
kubectl logs -n gc-system deployment/gc-controller | grep "Deleting resource"

# Check controller metrics
kubectl port-forward -n gc-system service/gc-controller-metrics 8080:8080
curl http://localhost:8080/metrics | grep gc_resources_deleted_total
```

### Recovery Options

#### Option 1: Restore from Backup

If you have backups of deleted resources:

```bash
# Restore from etcd backup (requires cluster admin access)
# This is cluster-specific and depends on your backup solution

# Example: Restore from Velero backup
velero restore create --from-backup <backup-name> --include-resources <resource-types>
```

#### Option 2: Recreate Resources

If resources can be recreated:

```bash
# Recreate from GitOps (if using GitOps)
kubectl apply -f <resource-definitions>

# Or recreate manually
kubectl create -f <resource-yaml>
```

#### Option 3: Restore from External Systems

If resources were backed up to external systems:

- Restore from database backups
- Restore from object storage backups
- Restore from configuration management systems

### Prevention Measures

1. **Use Dry-Run Mode**: Test policies with `dryRun: true` first
2. **Start with Long TTLs**: Use conservative TTL values initially
3. **Use Label Selectors**: Limit scope with label selectors
4. **Monitor Deletions**: Set up alerts for abnormal deletion rates
5. **Regular Backups**: Maintain regular backups of critical resources

---

## Backup Strategies

### Policy Backup

#### Manual Backup

```bash
# Backup all GC policies
kubectl get garbagecollectionpolicies --all-namespaces -o yaml > gc-policies-backup-$(date +%Y%m%d).yaml

# Backup specific namespace policies
kubectl get garbagecollectionpolicies -n <namespace> -o yaml > gc-policies-<namespace>-backup.yaml
```

#### Automated Backup

Create a CronJob to backup policies regularly:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: gc-policy-backup
  namespace: gc-system
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: bitnami/kubectl:latest
            command:
            - /bin/sh
            - -c
            - |
              kubectl get garbagecollectionpolicies --all-namespaces -o yaml > /backup/gc-policies-$(date +%Y%m%d).yaml
              # Upload to S3 or other storage
              # aws s3 cp /backup/gc-policies-*.yaml s3://backup-bucket/gc-policies/
          restartPolicy: OnFailure
          volumes:
          - name: backup
            emptyDir: {}
```

### Resource Backup

#### Velero Backup

```yaml
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: daily-backup
  namespace: velero
spec:
  includedNamespaces:
  - '*'
  excludedResources:
  - events
  - events.events.k8s.io
  ttl: 720h0m0s
  storageLocation: default
  volumeSnapshotLocations:
  - default
```

#### etcd Backup

For cluster-level backups:

```bash
# Backup etcd (requires cluster admin access)
ETCDCTL_API=3 etcdctl snapshot save /backup/etcd-snapshot-$(date +%Y%m%d).db \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key
```

### Backup Retention Policy

- **Daily Backups**: Keep for 7 days
- **Weekly Backups**: Keep for 4 weeks
- **Monthly Backups**: Keep for 12 months
- **Yearly Backups**: Keep indefinitely

---

## Emergency Stop Procedures

### "Break Glass" Emergency Stop

In case of emergency, follow these steps:

#### Method 1: Scale Down Controller

```bash
# Fastest method - stops all GC operations immediately
kubectl scale deployment gc-controller -n gc-system --replicas=0
```

#### Method 2: Delete Deployment

```bash
# More drastic - completely removes controller
kubectl delete deployment gc-controller -n gc-system
```

#### Method 3: Disable Policies

```bash
# Disable all policies across all namespaces
kubectl get garbagecollectionpolicies --all-namespaces -o json | \
  jq -r '.items[] | "\(.metadata.namespace) \(.metadata.name)"' | \
  while read ns name; do
    kubectl delete garbagecollectionpolicy $name -n $ns
  done
```

#### Method 4: Network Policy Block

```yaml
# Block all network traffic to controller
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gc-controller-block-all
  namespace: gc-system
spec:
  podSelector:
    matchLabels:
      app: gc-controller
  policyTypes:
  - Ingress
  - Egress
  # No rules = block all traffic
```

### Emergency Stop Script

Create a script for quick emergency stops:

```bash
#!/bin/bash
# emergency-stop.sh

set -e

echo "🚨 EMERGENCY STOP - Stopping GC Controller"
echo "This will stop all garbage collection operations"

# Scale down controller
kubectl scale deployment gc-controller -n gc-system --replicas=0

# Wait for pods to terminate
kubectl wait --for=delete pod -l app=gc-controller -n gc-system --timeout=60s

echo "✅ GC Controller stopped"
echo "⚠️  Remember to investigate and fix policies before restarting"
```

### Restarting After Emergency Stop

1. **Investigate**: Review what caused the emergency stop
2. **Fix Policies**: Correct or remove problematic policies
3. **Test**: Verify policies in dry-run mode
4. **Restart**: Scale controller back up

```bash
# Restart controller
kubectl scale deployment gc-controller -n gc-system --replicas=2

# Verify controller is running
kubectl get pods -n gc-system -l app=gc-controller
```

---

## Rollback Scenarios

### Rollback Controller Version

#### Method 1: Helm Rollback

```bash
# List releases
helm list -n gc-system

# Rollback to previous version
helm rollback gc-controller -n gc-system

# Rollback to specific revision
helm rollback gc-controller <revision-number> -n gc-system
```

#### Method 2: kubectl Rollout

```bash
# View rollout history
kubectl rollout history deployment/gc-controller -n gc-system

# Rollback to previous revision
kubectl rollout undo deployment/gc-controller -n gc-system

# Rollback to specific revision
kubectl rollout undo deployment/gc-controller -n gc-system --to-revision=<revision-number>
```

### Rollback Policy Changes

#### Restore from Backup

```bash
# Restore policy from backup
kubectl apply -f gc-policies-backup-20251221.yaml

# Or restore specific policy
kubectl apply -f gc-policy-<name>-backup.yaml
```

#### Git-based Rollback

If policies are managed via GitOps:

```bash
# Revert to previous commit
git revert <commit-hash>

# Or checkout previous version
git checkout <previous-commit-hash> -- <policy-file>
kubectl apply -f <policy-file>
```

### Rollback CRD Version

If CRD schema changes cause issues:

```bash
# List CRD versions
kubectl get crd garbagecollectionpolicies.gc.kube-zen.io -o yaml | grep -A 5 "versions:"

# Restore previous CRD version
kubectl apply -f deploy/crds/gc.kube-zen.io_garbagecollectionpolicies.yaml

# Migrate existing resources (if needed)
# This depends on the specific migration path
```

### Rollback Checklist

- [ ] Identify the issue causing rollback
- [ ] Determine rollback target (version/revision)
- [ ] Backup current state
- [ ] Execute rollback procedure
- [ ] Verify rollback success
- [ ] Monitor for issues
- [ ] Document rollback reason and procedure

---

## Disaster Recovery Testing

### Regular Testing Schedule

- **Monthly**: Test emergency stop procedure
- **Quarterly**: Test full disaster recovery
- **Annually**: Test complete cluster recovery

### Test Scenarios

1. **Mass Deletion Recovery**: Simulate accidental mass deletion
2. **Controller Failure**: Test recovery from controller crash
3. **Policy Corruption**: Test recovery from corrupted policies
4. **Network Partition**: Test behavior during network issues
5. **API Server Failure**: Test recovery from API server issues

### Testing Procedure

```bash
# 1. Create test environment
kubectl create namespace disaster-recovery-test

# 2. Deploy test resources
kubectl apply -f test-resources.yaml

# 3. Create test policy
kubectl apply -f test-policy.yaml

# 4. Simulate disaster
# (e.g., delete resources, corrupt policies, etc.)

# 5. Execute recovery procedure
./emergency-stop.sh
# ... recovery steps ...

# 6. Verify recovery
kubectl get all -n disaster-recovery-test

# 7. Cleanup
kubectl delete namespace disaster-recovery-test
```

---

## Recovery Time Objectives (RTO) and Recovery Point Objectives (RPO)

### Recommended Targets

- **RTO (Recovery Time Objective)**: < 15 minutes
  - Time to stop accidental deletions
  - Time to restore controller functionality

- **RPO (Recovery Point Objective)**: < 1 hour
  - Maximum acceptable data loss
  - Backup frequency should support this

### Achieving RTO/RPO

1. **Automated Backups**: Reduce manual intervention
2. **Documented Procedures**: Speed up recovery
3. **Regular Testing**: Ensure procedures work
4. **Monitoring**: Detect issues quickly
5. **Alerting**: Notify team immediately

---

## Summary

Disaster recovery for zen-gc requires:

1. **Prevention**: Use dry-run, monitoring, and careful policy design
2. **Detection**: Monitor deletion rates and set up alerts
3. **Response**: Have emergency stop procedures ready
4. **Recovery**: Maintain backups and know how to restore
5. **Testing**: Regularly test recovery procedures

Always have a plan, test it regularly, and keep backups current.

