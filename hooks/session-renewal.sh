#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# PreCompact. The runtime is about to renew this session's working context on
# its own threshold, and this is the only moment anything can say what the
# summary must keep. Stdout here becomes the runtime's custom compaction
# instructions verbatim — established in the Claude Code binary at 2.1.250,
# where executePreCompactHooks joins each succeeding hook's stdout and appends
# it to the summary request.
#
# Exit status is the whole contract and it is one-sided: 2 blocks the
# compaction, and an unattended session whose compaction is blocked runs into
# the context limit and stops. So this hook exits 0 whatever happens. A missing
# binary, an unreadable record, a lane it cannot resolve — each one means the
# runtime summarizes on its own judgment, which is what happens today.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

trigger=""
if [[ -n "$payload" ]] && command -v python3 >/dev/null 2>&1; then
  trigger=$(printf '%s' "$payload" | python3 -c 'import json,sys
try: print(json.load(sys.stdin).get("trigger",""))
except Exception: pass' 2>/dev/null)
fi

# `run` replays the captured payload on stdin, which is where the lane's
# working directory comes from.
run renew -trigger "$trigger" | bounded 9200
exit 0
