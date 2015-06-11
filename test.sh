#!/bin/bash
make build

docker run \
    -e HOUND_GRAPHITE_BASE=http://graphite.thraxil.org/render/ \
     -e HOUND_CARBON_BASE=griffin.thraxil.org:2003 \
     -e HOUND_METRIC_BASE=ccnmtl.app.gauges.hound. \
     -e HOUND_EMAIL_FROM=hound@thraxil.org \
     -e HOUND_EMAIL_TO=anders@columbia.edu \
     -e HOUND_CHECK_INTERVAL=1 \
     -e HOUND_GLOBAL_THROTTLE=10 \
     -e HOUND_EMAIL_ON_ERROR=false \
     --link postfix:postfix \
     -v /home/anders/code/go/src/github.com/ccnmtl/hound/config.json:/etc/hound/config.json \
     ccnmtl/hound
