#!/bin/bash

export HOUND_GRAPHITE_BASE="http://nanny.cul.columbia.edu/render/"
export HOUND_CARBON_BASE="nanny.cul.columbia.edu:2003"
export HOUND_METRIC_BASE="ccnmtl.app.gauges.hounddev."
export HOUND_EMAIL_FROM="hound@ccnmtl.columbia.edu"
export HOUND_EMAIL_TO="anders@columbia.edu"
export HOUND_CHECK_INTERVAL=5
export HOUND_GLOBAL_THROTTLE=10
export HOUND_HTTP_PORT=9998
export HOUND_TEMPLATE_FILE="index.html"
export HOUND_EMAIL_ON_ERROR=false
export HOUND_SMTP_SERVER=postgres
export HOUND_SMTP_PORT=25
export HOUND_LOG_LEVEL=DEBUG

./hound -config=config.json
