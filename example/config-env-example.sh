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

# Telegram notification settings
# export TELEGRAM_HANDLER_ENABLED="false"   # Enable Telegram notifications
# export TELEGRAM_BOT_TOKEN=""              # Telegram bot token (get from @BotFather)
# export TELEGRAM_CHAT_ID=""                # Telegram chat ID (group/channel/user)
# export TELEGRAM_TIMEOUT="10s"             # HTTP timeout for Telegram API
# export TELEGRAM_SEND_RAW="false"          # Send raw event data in addition to formatted message

# Telegram Setup Instructions:
# 1. Create a bot: Message @BotFather on Telegram, send /newbot, follow instructions
# 2. Get chat ID:
#    - For private chat: Start chat with bot, send message, visit:
#      https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates
#    - For group: Add bot to group, send message mentioning bot, check same URL
# 3. Test connection:
#    curl -X POST "https://api.telegram.org/bot<BOT_TOKEN>/sendMessage" \
#      -H "Content-Type: application/json" \
#      -d '{"chat_id": "<CHAT_ID>", "text": "Test from evacuator"}'
#
# TELEGRAM_SEND_RAW=true: Enables EMERGENCY FAILSAFE mode with multi-layer fallbacks:
# 1. Sends raw data FIRST (before any processing that could fail)
# 2. If JSON marshaling fails → sends structured text fallback
# 3. If everything fails → sends basic emergency capture
# GUARANTEES incident data is captured even during catastrophic parsing failures.
# ESSENTIAL for production environments - prevents data loss during critical events.

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
# 6. With Telegram notifications:
#    TELEGRAM_HANDLER_ENABLED=true \
#    TELEGRAM_BOT_TOKEN="bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11" \
#    TELEGRAM_CHAT_ID="-100123456789" \
#    ./evacuator
#
# 7. With Telegram notifications and raw data:
#    TELEGRAM_HANDLER_ENABLED=true \
#    TELEGRAM_BOT_TOKEN="bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11" \
#    TELEGRAM_CHAT_ID="-100123456789" \
#    TELEGRAM_SEND_RAW=true \
#    ./evacuator
#
# 8. Complete setup with all handlers:
#    LOG_LEVEL=info \
#    KUBERNETES_HANDLER_ENABLED=true \
#    TELEGRAM_HANDLER_ENABLED=true \
#    TELEGRAM_BOT_TOKEN="bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11" \
#    TELEGRAM_CHAT_ID="-100123456789" \
#    TELEGRAM_SEND_RAW=true \
#    ./evacuator
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
#   - name: TELEGRAM_HANDLER_ENABLED
#     value: "true"
#   - name: TELEGRAM_BOT_TOKEN
#     valueFrom:
#       secretKeyRef:
#         name: evacuator-telegram
#         key: bot-token
#   - name: TELEGRAM_CHAT_ID
#     valueFrom:
#       secretKeyRef:
#         name: evacuator-telegram
#         key: chat-id