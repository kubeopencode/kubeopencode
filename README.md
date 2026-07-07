<p align="center">
  <img src="docs/images/logo.png" alt="KubeOpenCode Logo" width="500">
</p>

<p align="center">
  <em>Kubernetes-native Agent Platform for Teams and Enterprise</em>
</p>

<p align="center">
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/kubeopencode/kubeopencode"><img src="https://goreportcard.com/badge/github.com/kubeopencode/kubeopencode" alt="Go Report Card"></a>
</p>

> **Note**: KubeOpenCode builds on the excellent [OpenCode](https://opencode.ai) AI agent. OpenCode is great for individual developers — KubeOpenCode makes it work for **teams and enterprises** by adding governance, shared agent configurations, scale, and enterprise infrastructure integration. This is an independent project, not affiliated with the OpenCode team.

<br>

<p align="center">
  <img src="docs/images/screenshot-shadow.png" alt="KubeOpenCode UI" width="800">
</p>

<br>

<p align="center">
  <a href="https://youtu.be/H_m8PMFQppc"><img src="https://img.shields.io/badge/▶_Watch_Demo-red?style=for-the-badge&logo=youtube&logoColor=white" alt="Watch Demo"></a>
</p>

---

### Installation

```bash
kubectl create namespace kubeopencode-system

helm install kubeopencode oci://ghcr.io/kubeopencode/helm-charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true
```

### Quick Example

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: default
  namespace: kubeopencode-system
spec:
  profile: "General-purpose development agent"
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  credentials:
    - name: api-key
      secretRef:
        name: ai-credentials
        key: api-key
      env: OPENCODE_API_KEY
---
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: my-task
  namespace: kubeopencode-system
spec:
  agentRef:
    name: default
  description: |
    Update dependencies to latest versions.
    Run tests and create PR.
```

```bash
# Attach to a live agent from your terminal
kubeoc agent attach default -n kubeopencode-system
```

### Documentation

For full documentation — including getting started, architecture, features, security, and more — [**head over to our docs**](website/docs/getting-started.md).

### CLI

```bash
go install github.com/kubeopencode/kubeopencode/cmd/kubeoc@latest

kubeoc get agents                                    # List agents
kubeoc agent attach my-agent -n kubeopencode-system  # Attach to an agent
```

### Community

- **Discussions**: [GitHub Discussions](https://github.com/kubeopencode/kubeopencode/discussions)
- **Issues**: [GitHub Issues](https://github.com/kubeopencode/kubeopencode/issues)

### Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### License

MIT License — see [LICENSE](LICENSE) for details.

---

Made with love by the KubeOpenCode community
