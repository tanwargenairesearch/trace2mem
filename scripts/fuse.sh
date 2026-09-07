#!/bin/sh
set -eu
: "${TRACE2MEM_SPACE:?Set TRACE2MEM_SPACE to a compiled fixture space}"
export TRACE2MEM_TOKEN="$(cat /data/secrets/.local/bootstrap-token)"
mkdir -p /tmp/trace2mem-mount
trace2mem mount --space "$TRACE2MEM_SPACE" --target /tmp/trace2mem-mount &
pid=$!
trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true' EXIT
n=0
until test -f /tmp/trace2mem-mount/knowledge/index.md; do
 n=$((n+1)); test "$n" -lt 60; sleep 1
done
ls -R /tmp/trace2mem-mount
stat /tmp/trace2mem-mount/knowledge/index.md
cat /tmp/trace2mem-mount/knowledge/index.md
grep -R 'cite:' /tmp/trace2mem-mount/knowledge
if echo forbidden > /tmp/trace2mem-mount/knowledge/index.md; then
 echo 'read-only guarantee failed' >&2; exit 1
fi
