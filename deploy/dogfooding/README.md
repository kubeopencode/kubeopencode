# KubeTask Dogfooding Environment

This directory contains resources for running KubeTask in a dogfooding environment, where KubeTask is used to automate tasks on its own repository.

## Architecture

```
GitHub (kubetask-io/kubetask)
    │
    │ Webhook events (PR opened, issue comments, etc.)
    │
    ▼
smee.io (https://smee.io/YOUR_CHANNEL_ID)
    │
    │ Forwards webhooks (trusted SSL)
    │
    ▼
smee-client (Pod in kubetask-system)
    │
    │ HTTP to internal service
    │
    ▼
kubetask-webhook (Service in kubetask-system)
    │
    │ Matches rules, creates Tasks
    │
    ▼
WebhookTrigger (kubetask-dogfooding/github)
    │
    ▼
Task → Job → Agent Pod
```

## Why smee.io?

GitHub webhooks require endpoints with SSL certificates signed by trusted public CAs. OpenShift's default ingress uses self-signed certificates, which GitHub rejects with:

```
tls: failed to verify certificate: x509: certificate signed by unknown authority
```

[smee.io](https://smee.io) is GitHub's recommended solution for development/testing. It provides a public HTTPS endpoint with trusted certificates and forwards webhooks to your internal services.

## Directory Structure

```
deploy/dogfooding/
├── README.md                 # This file
├── base/                     # Resources for kubetask-dogfooding namespace
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── rbac.yaml
│   ├── secrets.yaml          # Contains github-webhook-secret
│   ├── agent-default.yaml    # Default Agent configuration
│   └── context-*.yaml        # Context resources
├── system/                   # Resources for kubetask-system namespace
│   ├── kustomization.yaml
│   ├── deployment-smee-client.yaml  # Smee.io webhook proxy
│   └── route-webhook.yaml    # OpenShift Route (optional)
├── resources/                # WebhookTrigger definitions
│   └── webhooktrigger-github.yaml
└── examples/                 # Example Tasks
```

## Setup

### Prerequisites

1. KubeTask installed in `kubetask-system` namespace with webhook enabled:
   ```bash
   helm install kubetask ./charts/kubetask \
     --namespace kubetask-system \
     --set webhook.enabled=true
   ```

2. A GitHub App configured for the repository (see [GitHub App Setup](#github-app-setup))

### Deploy Dogfooding Resources

```bash
# Apply base resources (kubetask-dogfooding namespace)
kubectl apply -k deploy/dogfooding/base

# Apply system resources (kubetask-system namespace)
kubectl apply -k deploy/dogfooding/system

# Apply WebhookTrigger
kubectl apply -f deploy/dogfooding/resources/webhooktrigger-github.yaml
```

### Verify Deployment

```bash
# Check smee-client is running
kubectl get pods -n kubetask-system -l app.kubernetes.io/name=smee-client

# Check smee-client logs
kubectl logs -n kubetask-system -l app.kubernetes.io/name=smee-client

# Check webhook server registered the trigger
kubectl logs -n kubetask-system -l app.kubernetes.io/component=webhook | grep "Registered"
```

## GitHub App Setup

### 1. Create a GitHub App

1. Go to GitHub Settings → Developer settings → GitHub Apps → New GitHub App
2. Configure:
   - **App name**: `kubetask-bot`
   - **Homepage URL**: `https://github.com/kubetask-io/kubetask`
   - **Webhook URL**: `https://smee.io/YOUR_CHANNEL_ID` (from your smee.io channel)
   - **Webhook secret**: Same as `hmacKey` in `github-webhook-secret`
   - **Permissions**:
     - Repository: Contents (Read & Write), Issues (Read & Write), Pull requests (Read & Write)
   - **Subscribe to events**: Issue comment, Pull request

### 2. Install the App

Install the GitHub App on the `kubetask-io/kubetask` repository.

### 3. Configure Secrets

Create the webhook secret:
```bash
kubectl create secret generic github-webhook-secret \
  --namespace kubetask-dogfooding \
  --from-literal=hmacKey=<your-webhook-secret>
```

## Changing the smee.io Channel

If you need to create a new smee.io channel:

1. Go to https://smee.io/ and click "Start a new channel"
2. Update `system/deployment-smee-client.yaml` with the new URL
3. Update the GitHub App's Webhook URL
4. Re-apply the deployment:
   ```bash
   kubectl apply -k deploy/dogfooding/system
   ```

## WebhookTrigger Rules

The `github` WebhookTrigger in `resources/webhooktrigger-github.yaml` defines:

| Rule | Event | Trigger Condition |
|------|-------|-------------------|
| `pr-opened` | `pull_request` | PR is opened |
| `comment-privileged` | `issue_comment` | `@kubetask-bot` mention from OWNER/MEMBER/CONTRIBUTOR/COLLABORATOR |
| `comment-unprivileged` | `issue_comment` | `@kubetask-bot` mention from other users |

## Troubleshooting

### Webhook not triggering

1. **Check smee-client logs**:
   ```bash
   kubectl logs -n kubetask-system -l app.kubernetes.io/name=smee-client -f
   ```

2. **Check webhook server logs**:
   ```bash
   kubectl logs -n kubetask-system -l app.kubernetes.io/component=webhook -f
   ```

3. **Check GitHub App delivery history**:
   Go to GitHub App settings → Advanced → Recent Deliveries

### Authentication failed

If you see `Authentication failed` in webhook logs:
- Verify the `hmacKey` in `github-webhook-secret` matches the GitHub App's webhook secret
- Ensure the secret is in the correct namespace (`kubetask-dogfooding`)

### No Tasks created

Check the WebhookTrigger status:
```bash
kubectl get webhooktrigger -n kubetask-dogfooding github -o yaml
```

Check if the filter conditions match your event.

## Production Considerations

For production environments, consider:

1. **Use a trusted SSL certificate** instead of smee.io:
   - Configure Let's Encrypt with cert-manager
   - Or use a commercial SSL certificate

2. **Use a dedicated Route/Ingress** with proper TLS:
   ```yaml
   spec:
     tls:
       termination: edge
       certificate: <your-cert>
       key: <your-key>
   ```

3. **Secure the webhook secret** using external secret management (e.g., Vault, Sealed Secrets)
