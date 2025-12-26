# Secret Management Guide

This document describes how to securely manage sensitive configuration and secrets for zen-gc.

## Overview

zen-gc requires sensitive configuration for:
- **Webhook TLS Certificates**: TLS certificates and private keys for the validating webhook server
- **Kubernetes Service Account Tokens**: Automatically managed by Kubernetes
- **Future**: API keys, external service credentials (if needed)

## Webhook TLS Certificates

The validating webhook server requires TLS certificates for secure communication with the Kubernetes API server.

### Option 1: Kubernetes Secrets (Recommended)

#### Using kubectl

Create a Kubernetes Secret with TLS certificates:

```bash
# Generate self-signed certificate (for testing only)
openssl req -x509 -newkey rsa:2048 -keyout tls.key -out tls.crt -days 365 -nodes \
  -subj "/CN=gc-controller-webhook.gc-system.svc"

# Create secret
kubectl create secret tls gc-controller-webhook-cert \
  --cert=tls.crt \
  --key=tls.key \
  -n gc-system
```

#### Using YAML Manifest

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gc-controller-webhook-cert
  namespace: gc-system
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-certificate>
  tls.key: <base64-encoded-private-key>
```

**To encode files:**
```bash
cat tls.crt | base64 -w 0
cat tls.key | base64 -w 0
```

#### Mount Secret in Deployment

The deployment should mount the secret:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gc-controller
  namespace: gc-system
spec:
  template:
    spec:
      containers:
      - name: gc-controller
        volumeMounts:
        - name: webhook-certs
          mountPath: /etc/webhook/certs
          readOnly: true
      volumes:
      - name: webhook-certs
        secret:
          secretName: gc-controller-webhook-cert
```

### Option 2: cert-manager (Production Recommended)

cert-manager automatically manages TLS certificates, including automatic renewal.

#### Install cert-manager

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Verify installation
kubectl get pods -n cert-manager
```

#### Create Certificate Issuer

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

#### Create Certificate Resource

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: gc-controller-webhook-cert
  namespace: gc-system
spec:
  secretName: gc-controller-webhook-cert
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
  - gc-controller-webhook.gc-system.svc
  - gc-controller-webhook.gc-system.svc.cluster.local
```

#### Configure ValidatingWebhookConfiguration

The webhook configuration should reference cert-manager:

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: gc-controller-validating-webhook
  annotations:
    cert-manager.io/inject-ca-from: gc-system/gc-controller-webhook-cert
spec:
  webhooks:
  - name: validate-gc-policy.gc.kube-zen.io
    clientConfig:
      service:
        name: gc-controller-webhook
        namespace: gc-system
        path: "/validate-gc-policy"
    # ... rest of configuration
```

cert-manager will:
- Automatically inject the CA certificate into the webhook configuration
- Renew certificates before expiration
- Update the secret automatically

### Option 3: Self-Signed Certificates (Development Only)

For local development or testing:

```bash
# Generate self-signed certificate
openssl req -x509 -newkey rsa:2048 -keyout tls.key -out tls.crt -days 365 -nodes \
  -subj "/CN=gc-controller-webhook.gc-system.svc" \
  -addext "subjectAltName=DNS:gc-controller-webhook.gc-system.svc,DNS:gc-controller-webhook.gc-system.svc.cluster.local"

# Create secret
kubectl create secret tls gc-controller-webhook-cert \
  --cert=tls.crt \
  --key=tls.key \
  -n gc-system
```

**⚠️ Warning**: Self-signed certificates should **never** be used in production.

## Secret Rotation

### Manual Rotation

#### Step 1: Generate New Certificate

```bash
# Generate new certificate
openssl req -x509 -newkey rsa:2048 -keyout tls-new.key -out tls-new.crt -days 365 -nodes \
  -subj "/CN=gc-controller-webhook.gc-system.svc"
```

#### Step 2: Update Secret

```bash
# Update secret with new certificate
kubectl create secret tls gc-controller-webhook-cert \
  --cert=tls-new.crt \
  --key=tls-new.key \
  -n gc-system \
  --dry-run=client -o yaml | kubectl apply -f -
```

#### Step 3: Restart Pods

```bash
# Restart pods to pick up new certificate
kubectl rollout restart deployment/gc-controller -n gc-system

# Verify pods are running
kubectl get pods -n gc-system -l app=gc-controller
```

### Automatic Rotation (cert-manager)

With cert-manager, certificates are automatically renewed before expiration. No manual intervention is required.

**Monitor certificate expiration:**
```bash
# Check certificate expiration
kubectl get certificate gc-controller-webhook-cert -n gc-system -o yaml

# Check cert-manager logs
kubectl logs -n cert-manager -l app.kubernetes.io/instance=cert-manager
```

## External Secret Managers

For advanced secret management, consider integrating with external secret managers:

### HashiCorp Vault

#### Install Vault CSI Driver

```bash
# Install Vault CSI driver
kubectl apply -f https://raw.githubusercontent.com/hashicorp/vault-csi-provider/main/deployment/install.yaml
```

#### Create SecretProviderClass

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: gc-controller-webhook-certs
  namespace: gc-system
spec:
  provider: vault
  parameters:
    vaultAddress: "https://vault.example.com:8200"
    roleName: "gc-controller"
    objects: |
      - objectName: "tls.crt"
        secretPath: "secret/data/gc-controller/webhook"
        secretKey: "cert"
      - objectName: "tls.key"
        secretPath: "secret/data/gc-controller/webhook"
        secretKey: "key"
  secretObjects:
  - secretName: gc-controller-webhook-cert
    type: kubernetes.io/tls
    data:
    - objectName: tls.crt
      key: tls.crt
    - objectName: tls.key
      key: tls.key
```

#### Mount in Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gc-controller
spec:
  template:
    spec:
      containers:
      - name: gc-controller
        volumeMounts:
        - name: webhook-certs
          mountPath: /etc/webhook/certs
          readOnly: true
      volumes:
      - name: webhook-certs
        csi:
          driver: secrets-store.csi.k8s.io
          readOnly: true
          volumeAttributes:
            secretProviderClass: gc-controller-webhook-certs
```

### AWS Secrets Manager

Use [External Secrets Operator](https://external-secrets.io/) to sync secrets from AWS Secrets Manager:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: gc-controller-webhook-cert
  namespace: gc-system
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: gc-controller-webhook-cert
    creationPolicy: Owner
  data:
  - secretKey: tls.crt
    remoteRef:
      key: gc-controller/webhook
      property: cert
  - secretKey: tls.key
    remoteRef:
      key: gc-controller/webhook
      property: key
```

### Google Secret Manager

Similar to AWS, use External Secrets Operator:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: gc-controller-webhook-cert
  namespace: gc-system
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: gcp-secret-manager
    kind: SecretStore
  target:
    name: gc-controller-webhook-cert
  data:
  - secretKey: tls.crt
    remoteRef:
      key: gc-controller-webhook-cert
      property: cert
  - secretKey: tls.key
    remoteRef:
      key: gc-controller-webhook-cert
      property: key
```

## Security Best Practices

### 1. Use Kubernetes Secrets

- ✅ Store secrets in Kubernetes Secrets, not in ConfigMaps
- ✅ Use `type: kubernetes.io/tls` for TLS certificates
- ✅ Mount secrets as read-only volumes
- ✅ Never commit secrets to version control

### 2. Restrict Access

```yaml
# Use RBAC to restrict who can access secrets
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: secret-reader
  namespace: gc-system
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["gc-controller-webhook-cert"]
  verbs: ["get"]
```

### 3. Encrypt Secrets at Rest

Enable encryption at rest for etcd:

```yaml
# In kube-apiserver configuration
--encryption-provider-config=/etc/kubernetes/encryption-config.yaml
```

### 4. Use cert-manager for Production

- ✅ Automatic certificate renewal
- ✅ Integration with Let's Encrypt
- ✅ No manual certificate management
- ✅ Automatic CA injection

### 5. Monitor Certificate Expiration

Set up alerts for certificate expiration:

```yaml
# Prometheus alert rule
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: gc-controller-cert-expiry
spec:
  groups:
  - name: gc-controller
    rules:
    - alert: WebhookCertificateExpiringSoon
      expr: cert_manager_certificate_expiration_timestamp_seconds{name="gc-controller-webhook-cert"} - time() < 86400 * 7
      for: 1h
      annotations:
        summary: "Webhook certificate expiring in 7 days"
```

### 6. Rotate Secrets Regularly

- **TLS Certificates**: Rotate before expiration (cert-manager handles this automatically)
- **Service Account Tokens**: Kubernetes rotates these automatically
- **API Keys**: Rotate every 90 days or as per your security policy

### 7. Use Separate Secrets per Environment

- **Development**: Use self-signed certificates
- **Staging**: Use cert-manager with staging issuer
- **Production**: Use cert-manager with production issuer

### 8. Audit Secret Access

Enable Kubernetes audit logging:

```yaml
# Audit policy
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
  resources:
  - group: ""
    resources: ["secrets"]
```

## Troubleshooting

### Issue: Webhook Certificate Errors

**Symptoms:**
- Webhook requests failing with certificate errors
- Logs show "x509: certificate signed by unknown authority"

**Solutions:**
```bash
# Check certificate in secret
kubectl get secret gc-controller-webhook-cert -n gc-system -o yaml

# Verify certificate
kubectl get secret gc-controller-webhook-cert -n gc-system -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout

# Check webhook configuration CA bundle
kubectl get validatingwebhookconfiguration gc-controller-validating-webhook -o yaml

# Restart pods
kubectl rollout restart deployment/gc-controller -n gc-system
```

### Issue: cert-manager Not Renewing Certificates

**Symptoms:**
- Certificates expiring soon
- cert-manager logs show errors

**Solutions:**
```bash
# Check certificate status
kubectl describe certificate gc-controller-webhook-cert -n gc-system

# Check cert-manager logs
kubectl logs -n cert-manager -l app.kubernetes.io/instance=cert-manager

# Check issuer status
kubectl describe clusterissuer letsencrypt-prod
```

### Issue: Secret Not Found

**Symptoms:**
- Pods failing to start
- Volume mount errors

**Solutions:**
```bash
# Verify secret exists
kubectl get secret gc-controller-webhook-cert -n gc-system

# Check deployment volume configuration
kubectl get deployment gc-controller -n gc-system -o yaml | grep -A 10 volumes

# Verify secret is in correct namespace
kubectl get secrets -n gc-system | grep webhook
```

## Examples

### Complete Deployment with cert-manager

See `examples/webhook-with-cert-manager.yaml` for a complete example.

### Complete Deployment with Manual Secrets

See `examples/webhook-with-manual-secrets.yaml` for a complete example.

## References

- [Kubernetes Secrets Documentation](https://kubernetes.io/docs/concepts/configuration/secret/)
- [cert-manager Documentation](https://cert-manager.io/docs/)
- [External Secrets Operator](https://external-secrets.io/)
- [HashiCorp Vault CSI Driver](https://developer.hashicorp.com/vault/docs/platform/k8s/csi)
- [Kubernetes Encryption at Rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)

