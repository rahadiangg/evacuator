#!/bin/bash
# REFERENCE ONLY - Environment Variable Examples
# 
# This file shows all supported environment variables for the evacuator application.
# DO NOT source this file directly - it's for documentation purposes only.
# 
# Copy the variables you need and set them in your deployment environment.

# Application settings
# export DRY_RUN="false"                    # Enable dry-run mode (no actual node draining)

# Cloud provider settings  
# export CLOUD_PROVIDER=""                  # Force specific provider: alibaba (empty = auto-detect)
# export AUTO_DETECT="true"                 # Enable auto-detection when CLOUD_PROVIDER is empty
# export POLL_INTERVAL="5s"                 # How often to check for spot termination events (3s-30s)
# export PROVIDER_TIMEOUT="5s"              # Timeout for cloud provider API calls (default: 5s)
# export PROVIDER_RETRIES="3"               # Number of retries for failed cloud provider requests (default: 3)

# Handler settings
# export LOG_HANDLER_ENABLED="true"         # Enable log handler
# export KUBERNETES_HANDLER_ENABLED="true"  # Enable Kubernetes handler

# Kubernetes settings  
# export NODE_NAME="$(hostname)"            # Node name (usually auto-set by DaemonSet downward API)
# export KUBECONFIG=""                      # Path to kubeconfig (leave empty for in-cluster)
# export KUBERNETES_IN_CLUSTER="true"       # Use in-cluster config (false = use kubeconfig)

# Logging settings
# export LOG_LEVEL="info"                   # debug, info, warn, error
# export LOG_FORMAT="json"                  # json, text

# Configuration file (optional)
# export CONFIG_FILE=""                     # Path to custom config file

# Usage examples:
# 1. Basic usage with dry-run:
#    DRY_RUN=true LOG_LEVEL=debug ./evacuator
#
# 2. With custom poll interval (3s-30s range):
#    POLL_INTERVAL=3s ./evacuator   # Fast detection
#    POLL_INTERVAL=10s ./evacuator  # Balanced detection  
#    POLL_INTERVAL=30s ./evacuator  # Conservative detection
#
# 3. With custom config file:
#    CONFIG_FILE="./my-config.yaml" ./evacuator
#
# 4. Force Alibaba Cloud provider:
#    CLOUD_PROVIDER=alibaba ./evacuator
#
# 5. Disable auto-detection:
#    CLOUD_PROVIDER=alibaba AUTO_DETECT=false ./evacuator
#
# RECOMMENDED USAGE:
# Most users should use environment variables directly in their deployment:
# 
# For Kubernetes DaemonSet:
# env:
#   - name: DRY_RUN
#     value: "false"
#   - name: LOG_LEVEL  
#     value: "info"
#   - name: NODE_NAME
#     valueFrom:
#       fieldRef:
#         fieldPath: spec.nodeName
#   - name: POLL_INTERVAL
#     value: "5s"