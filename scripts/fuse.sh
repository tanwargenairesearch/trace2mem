#!/bin/sh
set -eu
: "${BRAIN_SPACE:?Set BRAIN_SPACE to a compiled fixture space}"
export BRAIN_TOKEN="$(cat /data/secrets/.local/bootstrap-token)"
mkdir -p /tmp/brain-mount
brainctl mount --space "$BRAIN_SPACE" --target /tmp/brain-mount &
pid=$!
trap 'kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true' EXIT
n=0
until test -f /tmp/brain-mount/knowledge/index.md; do
 n=$((n+1)); test "$n" -lt 60; sleep 1
done
ls -R /tmp/brain-mount
stat /tmp/brain-mount/knowledge/index.md
cat /tmp/brain-mount/knowledge/index.md
grep -R 'cite:' /tmp/brain-mount/knowledge
if echo forbidden > /tmp/brain-mount/knowledge/index.md; then
 echo 'read-only guarantee failed' >&2; exit 1
fi
