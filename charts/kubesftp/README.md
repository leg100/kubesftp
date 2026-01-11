# KubeSFTP Helm Chart

A Helm chart for deploying a stateless SFTP/SSH server using OpenSSH on Kubernetes.

## Introduction

This chart bootstraps an SFTP server deployment on a Kubernetes cluster using the Helm package manager. It uses the `atmoz/sftp` Docker image which provides a simple and secure SFTP server based on OpenSSH.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- PV provisioner support in the underlying infrastructure (if persistence is enabled)

## Installing the Chart

To install the chart with the release name `my-sftp`:

```bash
helm install my-sftp charts/kubesftp
```

The command deploys an SFTP server on the Kubernetes cluster with default configuration. The [Parameters](#parameters) section lists the parameters that can be configured during installation.

## Uninstalling the Chart

To uninstall/delete the `my-sftp` deployment:

```bash
helm uninstall my-sftp
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Parameters

### Global parameters

| Name                      | Description                                     | Value           |
| ------------------------- | ----------------------------------------------- | --------------- |
| `replicaCount`            | Number of SFTP server replicas                  | `1`             |
| `nameOverride`            | String to partially override kubesftp.fullname  | `""`            |
| `fullnameOverride`        | String to fully override kubesftp.fullname      | `""`            |

### Image parameters

| Name                | Description                                | Value              |
| ------------------- | ------------------------------------------ | ------------------ |
| `image.repository`  | SFTP image repository                      | `atmoz/sftp`       |
| `image.tag`         | SFTP image tag                             | `alpine`           |
| `image.pullPolicy`  | SFTP image pull policy                     | `IfNotPresent`     |
| `imagePullSecrets`  | Specify docker-registry secret names       | `[]`               |

### Service parameters

| Name                       | Description                                    | Value            |
| -------------------------- | ---------------------------------------------- | ---------------- |
| `service.type`             | Kubernetes Service type                        | `LoadBalancer`   |
| `service.port`             | SFTP service port                              | `22`             |
| `service.loadBalancerIP`   | LoadBalancer IP (optional)                     | `""`             |
| `service.annotations`      | Service annotations                            | `{}`             |

### SFTP Configuration

| Name                | Description                                                  | Value                    |
| ------------------- | ------------------------------------------------------------ | ------------------------ |
| `sftpUsers`         | List of SFTP users in format: username:password:uid:gid     | `["demo:demo:1001:100"]` |

The `sftpUsers` parameter accepts a list of user definitions. Each user is defined in the format:
`username:password:uid:gid[:directory]`

Example:
```yaml
sftpUsers:
  - "user1:pass123:1001:100"
  - "user2:pass456:1002:100:upload"
  - "user3:pass789:1003:100:data"
```

### Persistence parameters

| Name                          | Description                                | Value              |
| ----------------------------- | ------------------------------------------ | ------------------ |
| `persistence.enabled`         | Enable persistence using PVC               | `true`             |
| `persistence.storageClass`    | PVC Storage Class                          | `""`               |
| `persistence.accessMode`      | PVC Access Mode                            | `ReadWriteOnce`    |
| `persistence.size`            | PVC Storage Request                        | `10Gi`             |
| `persistence.existingClaim`   | Use an existing PVC                        | `""`               |
| `persistence.annotations`     | PVC annotations                            | `{}`               |

### Security parameters

| Name                        | Description                                | Value       |
| --------------------------- | ------------------------------------------ | ----------- |
| `podSecurityContext`        | Pod security context                       | See values  |
| `securityContext`           | Container security context                 | See values  |
| `sshHostKeys.rsa`           | RSA host key (base64 encoded)              | `""`        |
| `sshHostKeys.dsa`           | DSA host key (base64 encoded)              | `""`        |
| `sshHostKeys.ecdsa`         | ECDSA host key (base64 encoded)            | `""`        |
| `sshHostKeys.ed25519`       | ED25519 host key (base64 encoded)          | `""`        |

### Service Account parameters

| Name                         | Description                                    | Value    |
| ---------------------------- | ---------------------------------------------- | -------- |
| `serviceAccount.create`      | Enable creation of ServiceAccount              | `true`   |
| `serviceAccount.name`        | Name of the created serviceAccount             | `""`     |
| `serviceAccount.annotations` | Service Account annotations                    | `{}`     |

### Other parameters

| Name              | Description                                | Value    |
| ----------------- | ------------------------------------------ | -------- |
| `podAnnotations`  | Pod annotations                            | `{}`     |
| `resources`       | CPU/Memory resource requests/limits        | `{}`     |
| `nodeSelector`    | Node labels for pod assignment             | `{}`     |
| `tolerations`     | Tolerations for pod assignment             | `[]`     |
| `affinity`        | Affinity for pod assignment                | `{}`     |

## Configuration Examples

### Example 1: Basic SFTP server with custom users

```yaml
sftpUsers:
  - "alice:password123:1001:100"
  - "bob:password456:1002:100"

persistence:
  size: 20Gi
```

### Example 2: SFTP server with existing PVC

```yaml
persistence:
  enabled: true
  existingClaim: my-existing-pvc
```

### Example 3: SFTP server with custom SSH host keys

First, generate the SSH host keys:
```bash
ssh-keygen -t rsa -f ssh_host_rsa_key -N ''
ssh-keygen -t ed25519 -f ssh_host_ed25519_key -N ''
```

Then configure the chart:
```yaml
sshHostKeys:
  rsa: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    [your key content here]
    -----END OPENSSH PRIVATE KEY-----
  ed25519: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    [your key content here]
    -----END OPENSSH PRIVATE KEY-----
```

### Example 4: NodePort service type

```yaml
service:
  type: NodePort
  port: 22
```

## Connecting to SFTP Server

After installing the chart, follow the instructions in the NOTES to get the service IP/address.

Connect using an SFTP client:

```bash
sftp -P 22 username@<SERVICE_IP>
```

Or using command line:

```bash
sftp username@<SERVICE_IP>
```

## Persistence

The SFTP server stores uploaded files in `/home` directory within the container. To persist data across pod restarts, persistence is enabled by default using a PersistentVolumeClaim.

You can:
- Use dynamic provisioning with a StorageClass
- Use an existing PersistentVolumeClaim
- Disable persistence (data will be lost on pod restart)

## Security Considerations

1. **Change default passwords**: The default configuration includes demo credentials. Always change these in production.
2. **Use SSH keys**: Consider using SSH key authentication instead of passwords.
3. **Persistent host keys**: For production, provide persistent SSH host keys to avoid "host key changed" warnings.
4. **Network policies**: Implement Kubernetes NetworkPolicies to restrict access.
5. **Resource limits**: Set appropriate resource limits for your workload.

## Troubleshooting

### Pod fails to start with permission errors

Check the `podSecurityContext` and `securityContext` settings. The SFTP server requires specific UID/GID settings.

### Cannot connect to SFTP server

1. Check if the service is running: `kubectl get svc`
2. Verify the LoadBalancer has an external IP (if using LoadBalancer type)
3. Check pod logs: `kubectl logs <pod-name>`
4. Verify firewall rules allow traffic on port 22

### Data not persisting

1. Check if PVC is bound: `kubectl get pvc`
2. Verify the storage class exists: `kubectl get storageclass`
3. Check PVC events: `kubectl describe pvc <pvc-name>`

## License

This Helm chart is licensed under the Mozilla Public License Version 2.0.
