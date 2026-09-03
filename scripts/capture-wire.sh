#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# What a headless session actually receives, measured rather than asked.
#
# Runs one `claude -p` against a loopback server that records the request body
# and answers 400, so the session stops at its first turn. Only the body is
# written: the headers carry the credential and never reach disk, and nothing
# is forwarded anywhere.
#
# Usage: scripts/capture-wire.sh [-model opus] [-settings FILE] [-out DIR]
#                                [-prompt FILE] [-port N]
# Prints one line per captured request and leaves the bodies in -out.
set -uo pipefail

model=opus
settings=""
out="${TMPDIR:-/tmp}/mellions-wire-$$"
prompt=""
port=8791

while [ $# -gt 0 ]; do
  case "$1" in
    -model)    model="$2"; shift 2 ;;
    -settings) settings="$2"; shift 2 ;;
    -out)      out="$2"; shift 2 ;;
    -prompt)   prompt="$2"; shift 2 ;;
    -port)     port="$2"; shift 2 ;;
    *) echo "capture-wire: unknown argument $1" >&2; exit 2 ;;
  esac
done

command -v claude >/dev/null || { echo "capture-wire: no claude on PATH" >&2; exit 2; }
mkdir -p "$out/bodies"

if [ -z "$prompt" ]; then
  prompt="$out/prompt.md"
  printf 'Say the word ok and stop.\n' > "$prompt"
fi

python3 - "$port" "$out/bodies" <<'PY' &
import http.server, os, sys, threading
port, out = int(sys.argv[1]), sys.argv[2]
n, lock = [0], threading.Lock()
BODY = b'{"type":"error","error":{"type":"invalid_request_error","message":"capture"}}'

class H(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    def log_message(self, *a): pass
    def _refuse(self):
        self.send_response(400)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(BODY)))
        self.end_headers()
        self.wfile.write(BODY)
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("content-length") or 0))
        with lock:
            n[0] += 1; i = n[0]
        # body only; the headers carry the credential and are never written
        open(os.path.join(out, "req-%03d.json" % i), "wb").write(body)
        self._refuse()
    def do_GET(self): self._refuse()

http.server.ThreadingHTTPServer(("127.0.0.1", port), H).serve_forever()
PY
proxy=$!
trap 'kill "$proxy" 2>/dev/null' EXIT
sleep 1

set -- -p --model "$model" --output-format stream-json --verbose
[ -n "$settings" ] && set -- "$@" --settings "$settings" --permission-mode acceptEdits
ANTHROPIC_BASE_URL="http://127.0.0.1:$port" ANTHROPIC_API_KEY=capture \
  claude "$@" < "$prompt" > "$out/session.out" 2> "$out/session.err"

kill "$proxy" 2>/dev/null; wait "$proxy" 2>/dev/null
trap - EXIT

python3 - "$out/bodies" <<'PY'
import json, glob, os, sys
found = False
for p in sorted(glob.glob(os.path.join(sys.argv[1], "req-*.json"))):
    try:
        o = json.load(open(p))
    except Exception:
        continue
    blocks = o.get("system")
    if not isinstance(blocks, list):
        continue
    found = True
    text = "\n".join(b.get("text", "") for b in blocks)
    open(p.replace(".json", ".system.txt"), "w").write(text)
    print("%s model=%s system=%d bytes tools=%d" % (
        os.path.basename(p), o.get("model"), len(text), len(o.get("tools", []))))
if not found:
    print("no request carried a system prompt — the session did not reach its first turn")
    raise SystemExit(1)
PY
echo "bodies and system prompts: $out/bodies"
