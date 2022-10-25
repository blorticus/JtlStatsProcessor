#!/bin/sh

/opt/jtl-stats-processor $@ 2>&1 | tee /tmp/run

wait
