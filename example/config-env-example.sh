#!/bin/bash
# REFERENCE ONLY - Environment Variable Examples
# 
# This file shows all supported environment variables for the evacuator application.
# DO NOT source this file directly - it's for documentation purposes only.
# 
# Copy the variables you need and set them in your deployment environment.
#
# The application now uses Viper for configuration management, which provides:
# - Automatic precedence: CLI flags > env vars > config file > defaults
# - Support for multiple config formats (YAML, JSON, TOML, etc.)
# - Environment variable prefix support (EVACUATOR_*)
# - Automatic type conversion
# - No manual merging required
#
# ENVIRONMENT VARIABLE FORMAT:
# Variables match the YAML structure exactly: section.key becomes SECTION_KEY
# You can optionally use the EVACUATOR_ prefix for any variable
# Examples: APP_DRY_RUN or EVACUATOR_APP_DRY_RUN

# ===== ENVIRONMENT VARIABLES =====

# APPLICATION SETTINGS
# export APP_DRY_RUN="false"                    # app.dry_run - Enable dry-run mode
# export APP_NODE_NAME=""                       # app.node_name - Node name to monitor (auto-detected)

# MONITORING SETTINGS  
# export MONITORING_PROVIDER=""                 # monitoring.provider - Force provider: alibaba
# export MONITORING_AUTO_DETECT="true"          # monitoring.auto_detect - Auto-detect provider
# export MONITORING_POLL_INTERVAL="5s"          # monitoring.poll_interval - Check interval (3s-30s)
# export MONITORING_PROVIDER_TIMEOUT="3s"       # monitoring.provider_timeout - API call timeout
# export MONITORING_PROVIDER_RETRIES="3"        # monitoring.provider_retries - Retry attempts
# export MONITORING_EVENT_BUFFER_SIZE="100"     # monitoring.event_buffer_size - Event buffer

# HANDLER SETTINGS
# export HANDLERS_LOG_ENABLED="true"            # handlers.log.enabled - Enable log handler
# export HANDLERS_KUBERNETES_ENABLED="true"     # handlers.kubernetes.enabled - Enable k8s handler

# KUBERNETES DRAIN SETTINGS
# export HANDLERS_KUBERNETES_DRAIN_TIMEOUT_SECONDS="90"        # handlers.kubernetes.drain_timeout_seconds
# export HANDLERS_KUBERNETES_FORCE_EVICTION_AFTER="90s"        # handlers.kubernetes.force_eviction_after
# export HANDLERS_KUBERNETES_SKIP_DAEMON_SETS="true"           # handlers.kubernetes.skip_daemon_sets
# export HANDLERS_KUBERNETES_DELETE_EMPTY_DIR_DATA="false"     # handlers.kubernetes.delete_empty_dir_data
# export HANDLERS_KUBERNETES_IGNORE_POD_DISRUPTION="true"      # handlers.kubernetes.ignore_pod_disruption
# export HANDLERS_KUBERNETES_GRACE_PERIOD_SECONDS="10"         # handlers.kubernetes.grace_period_seconds
# export HANDLERS_KUBERNETES_KUBECONFIG=""                     # handlers.kubernetes.kubeconfig - Kubeconfig path
# export HANDLERS_KUBERNETES_IN_CLUSTER="true"                 # handlers.kubernetes.in_cluster - Use in-cluster config

# TELEGRAM NOTIFICATION SETTINGS
# export HANDLERS_TELEGRAM_ENABLED="false"      # handlers.telegram.enabled - Enable Telegram
# export HANDLERS_TELEGRAM_BOT_TOKEN=""         # handlers.telegram.bot_token - Bot token
# export HANDLERS_TELEGRAM_CHAT_ID=""           # handlers.telegram.chat_id - Chat ID
# export HANDLERS_TELEGRAM_TIMEOUT="10s"        # handlers.telegram.timeout - HTTP timeout
# export HANDLERS_TELEGRAM_SEND_RAW="false"     # handlers.telegram.send_raw - Send raw data

# LOGGING SETTINGS
# export LOGGING_LEVEL="info"                   # logging.level - Log level (debug/info/warn/error)
# export LOGGING_FORMAT="json"                  # logging.format - Log format (json/text)
# export LOGGING_OUTPUT="stdout"                # logging.output - Log output destination

# CONFIGURATION FILE (optional)
# export CONFIG_FILE=""                         # Path to custom config file

# ===== EVACUATOR_ PREFIX (OPTIONAL) =====
# All above variables can also be prefixed with EVACUATOR_:
# Examples:
# export EVACUATOR_APP_DRY_RUN="false"
# export EVACUATOR_MONITORING_PROVIDER="alibaba"
# export EVACUATOR_HANDLERS_KUBERNETES_KUBECONFIG="/path/to/kubeconfig"
# export EVACUATOR_HANDLERS_KUBERNETES_IN_CLUSTER="true"
# export EVACUATOR_LOGGING_LEVEL="info"

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

# USAGE EXAMPLES:

# 1. Basic usage:
#    APP_DRY_RUN=true LOGGING_LEVEL=debug ./evacuator
#    MONITORING_PROVIDER=alibaba HANDLERS_TELEGRAM_ENABLED=true ./evacuator

# 2. With EVACUATOR_ prefix:
#    EVACUATOR_APP_DRY_RUN=true EVACUATOR_LOGGING_LEVEL=debug ./evacuator

# 3. With custom config file:
#    CONFIG_FILE="./my-config.yaml" ./evacuator

# 4. Complete example with Telegram:
#    APP_DRY_RUN=false \
#    MONITORING_PROVIDER=alibaba \
#    HANDLERS_TELEGRAM_ENABLED=true \
#    HANDLERS_TELEGRAM_BOT_TOKEN="bot123456:ABC-DEF" \
#    HANDLERS_TELEGRAM_CHAT_ID="-100123456789" \
#    LOGGING_LEVEL=info \
#    ./evacuator

# KUBERNETES DAEMONSET EXAMPLE:
# env:
#   - name: APP_DRY_RUN
#     value: "false"
#   - name: LOGGING_LEVEL  
#     value: "info"
#   - name: LOGGING_FORMAT
#     value: "json"
#   - name: APP_NODE_NAME
#     valueFrom:
#       fieldRef:
#         fieldPath: spec.nodeName
#   - name: MONITORING_POLL_INTERVAL
#     value: "5s"
#   - name: HANDLERS_TELEGRAM_ENABLED
#     value: "true"
#   - name: HANDLERS_TELEGRAM_BOT_TOKEN
#     valueFrom:
#       secretKeyRef:
#         name: evacuator-telegram
#         key: bot-token
#   - name: HANDLERS_TELEGRAM_CHAT_ID
#     valueFrom:
#       secretKeyRef:
#         name: evacuator-telegram
#         key: chat-id