# Evacuator

A cloud instance termination monitor with pluggable handlers for graceful workload management.

## Overview

Evacuator monitors cloud provider instance metadata for termination events (spot instances, maintenance, autoscaling) and executes configurable handlers to gracefully manage workloads before the instance terminates. It supports multiple cloud providers with automatic detection and includes handlers for Kubernetes node draining and Telegram notifications.

## Features

- **Multi-cloud Support**: AWS, Google Cloud Platform, AliCloud, Tencent Cloud, Huawei Cloud, and Dummy (for testing)
- **Automatic Provider Detection**: Detects cloud provider from instance metadata
- **Pluggable Handlers**: Extensible handler system for different workload management strategies
- **Kubernetes Integration**: Built-in handler for cordoning and draining nodes gracefully
- **HashiCorp Nomad Integration**: Built-in handler for draining Nomad nodes gracefully
- **Telegram Notifications**: Handler for sending alerts when termination events are detected
- **Flexible Configuration**: Environment variables, YAML config files, or default values
- **Configurable Processing Timeout**: Default 75s recommended for AWS 2-minute termination window, adjustable for other providers (e.g., GCP 30s window)

## Installation

### Docker Images

Available on Docker Hub with multi-architecture support (linux/amd64, linux/arm64):
- Latest: `rahadiangg/evacuator:latest`
- Versioned: `rahadiangg/evacuator:1.2.3` (without "v" prefix)

### Binary Downloads

Pre-compiled binaries available from [GitHub Releases](https://github.com/rahadiangg/evacuator/releases):
- `evacuator-v{version}-linux-amd64.zip` (with "v" prefix)
- `evacuator-v{version}-linux-arm64.zip` (with "v" prefix)

## Deployment Examples

### Kubernetes DaemonSet

See [`example/k8s-daemonset.yaml`](example/k8s-daemonset.yaml) for a complete Kubernetes deployment example with RBAC configuration.

```bash
kubectl apply -f example/k8s-daemonset.yaml
```

## Supported Cloud Providers

| Provider | Termination Detection |
|----------|----------------------|
| **AWS** | Spot instance termination |
| **Google Cloud** | Preemptible instance termination |
| **AliCloud** | Spot instance termination |
| **Tencent Cloud** | Spot instance termination |
| **Huawei Cloud** | Spot instance termination |
| **Dummy** | Testing and development |

## Testing

```bash
# Run with dummy provider for testing
docker run -d \
  --name evacuator-test \
  -e PROVIDER_NAME=dummy \
  -e PROVIDER_DUMMY_DETECTION_WAIT=5s \
  -e KUBERNETES_ENABLED=false \
  -e LOG_LEVEL=debug \
  rahadiangg/evacuator:latest
```

## Configuration

Configuration follows precedence order (highest to lowest):
1. **Environment variables**
2. **YAML configuration file**
3. **Default values**

### Environment Variables

Environment variables use uppercase with underscores. Nested YAML keys use underscores as separators:

| Environment Variable | YAML Path | Default | Description |
|---------------------|-----------|---------|-------------|
| `NODE_NAME` | `node_name` | `""` | Node name (auto-detected if empty) |
| `PROVIDER_NAME` | `provider.name` | `""` | Cloud provider name (aws, gcp, alicloud, tencent, huawei, dummy) |
| `PROVIDER_AUTO_DETECT` | `provider.auto_detect` | `true` | Auto-detect cloud provider |
| `PROVIDER_POLL_INTERVAL` | `provider.poll_interval` | `"3s"` | Metadata polling interval |
| `PROVIDER_REQUEST_TIMEOUT` | `provider.request_timeout` | `"2s"` | Metadata request timeout |
| `PROVIDER_DUMMY_DETECTION_WAIT` | `provider.dummy.detection_wait` | `"10s"` | Dummy provider detection delay |
| `HANDLER_PROCESSING_TIMEOUT` | `handler.processing_timeout` | `"75s"` | Handler processing timeout |
| `HANDLER_KUBERNETES_ENABLED` | `handler.kubernetes.enabled` | `false` | Enable Kubernetes node draining |
| `HANDLER_KUBERNETES_SKIP_DAEMON_SETS` | `handler.kubernetes.skip_daemon_sets` | `true` | Skip DaemonSet pods during drain |
| `HANDLER_KUBERNETES_DELETE_EMPTY_DIR_DATA` | `handler.kubernetes.delete_empty_dir_data` | `false` | Delete pods with emptyDir volumes |
| `HANDLER_KUBERNETES_KUBECONFIG` | `handler.kubernetes.kubeconfig` | `""` | Path to kubeconfig file |
| `HANDLER_KUBERNETES_IN_CLUSTER` | `handler.kubernetes.in_cluster` | `true` | Use in-cluster service account |
| `HANDLER_NOMAD_ENABLED` | `handler.nomad.enabled` | `false` | Enable Nomad node draining |
| `HANDLER_NOMAD_FORCE` | `handler.nomad.force` | `false` | Force drain the node (ignore errors) |
| `HANDLER_TELEGRAM_ENABLED` | `handler.telegram.enabled` | `false` | Enable Telegram notifications |
| `HANDLER_TELEGRAM_BOT_TOKEN` | `handler.telegram.bot_token` | `""` | Telegram bot token |
| `HANDLER_TELEGRAM_CHAT_ID` | `handler.telegram.chat_id` | `""` | Telegram chat/channel ID |
| `LOG_LEVEL` | `log.level` | `"info"` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `log.format` | `"json"` | Log format (json, text) |

### YAML Configuration

See [`example/config-example.yaml`](example/config-example.yaml) for complete configuration options.

```bash
./evacuator --config example/config-example.yaml
```

## Support

For issues and questions:
- Create an issue on GitHub
- Check the logs with `LOG_LEVEL=debug` for troubleshooting
