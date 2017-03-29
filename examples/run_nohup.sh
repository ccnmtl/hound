#!/bin/sh

set -e

make hound

echo "Running hound..."

HOUND_GRAPHITE_BASE="https://graphite.example.com/render/" \
HOUND_CARBON_BASE="graphite.example.com:2003" \
HOUND_METRIC_BASE="apps.gauges.hounddev." \
HOUND_EMAIL_FROM="hound@example.com" \
HOUND_EMAIL_TO="you@example.com" \
HOUND_CHECK_INTERVAL=1 \
HOUND_GLOBAL_THROTTLE=10 \
HOUND_HTTP_PORT=9998 \
HOUND_TEMPLATE_FILE="index.html" \
HOUND_ALERT_TEMPLATE_FILE="alert.html" \
HOUND_EMAIL_ON_ERROR=false \
HOUND_SMTP_SERVER=postgres \
HOUND_SMTP_PORT=25 \
HOUND_LOG_LEVEL=DEBUG \
\
>hound.out 2>hound.err </dev/null nohup ./hound -config=config.json &
