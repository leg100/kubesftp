# kubesftp

Stateless SFTP/SSH server for Kubernetes deployed via Helm chart.

## Overview

This repository provides a Helm chart for deploying a secure, stateless SFTP server on Kubernetes using OpenSSH. The solution is designed to be simple, secure, and production-ready.

## Features

- 🚀 Easy deployment via Helm
- 🔒 Secure SFTP access using OpenSSH
- 💾 Persistent storage support
- 🔑 Configurable user authentication
- 📦 Stateless design for high availability
- ⚙️ Highly configurable via Helm values
- 🔐 Optional SSH host key persistence

## Quick Start

### Prerequisites

- Kubernetes cluster (1.19+)
- Helm 3.2.0+
- kubectl configured to access your cluster

### Installation

1. Clone this repository:
```bash
git clone https://github.com/leg100/kubesftp.git
cd kubesftp
```

2. Install the Helm chart:
```bash
helm install my-sftp charts/kubesftp
```

3. Get the SFTP server address:
```bash
kubectl get svc
```

4. Connect to the SFTP server:
```bash
sftp -P 22 demo@<EXTERNAL-IP>
# Password: demo
```

## Configuration

The Helm chart is highly configurable. See [charts/kubesftp/README.md](charts/kubesftp/README.md) for detailed configuration options.

### Common Configuration Examples

**Custom users:**
```yaml
# values.yaml
sftpUsers:
  - "alice:password123:1001:100"
  - "bob:password456:1002:100"
```

**Custom storage size:**
```yaml
# values.yaml
persistence:
  size: 50Gi
```

**NodePort service:**
```yaml
# values.yaml
service:
  type: NodePort
```

Install with custom values:
```bash
helm install my-sftp charts/kubesftp -f values.yaml
```

## Architecture

The SFTP server uses:
- **Base Image**: `atmoz/sftp` (Alpine-based OpenSSH server)
- **Storage**: Kubernetes PersistentVolumeClaim
- **Networking**: Kubernetes Service (LoadBalancer by default)
- **Security**: Pod security contexts and configurable SSH host keys

## Documentation

- [Helm Chart README](charts/kubesftp/README.md) - Detailed configuration guide
- [Chart Values](charts/kubesftp/values.yaml) - All available configuration options

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the Mozilla Public License Version 2.0 - see the [LICENSE](LICENSE) file for details.
