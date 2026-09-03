#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
set -euo pipefail

here=$(cd "$(dirname "$0")/.." && pwd)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/mellions-schedulers.XXXXXX")
scratch=$(cd "$scratch" && pwd -P)
trap 'rm -rf "$scratch"' EXIT

checkout="$scratch/Acme & Co 50%/checkout"
user_home="$scratch/user home"
mellions_home="$scratch/state and reports"
workdir="$scratch/work area"
claude_bin="$scratch/bin/claude"
output="$scratch/rendered"

python3 "$here/deploy/render_schedulers.py" \
  --output-dir "$output" \
  --checkout "$checkout" \
  --user-home "$user_home" \
  --mellions-home "$mellions_home" \
  --workdir "$workdir" \
  --path '/opt/acme & co/bin:/usr/bin:/bin' \
  --claude-bin "$claude_bin" \
  --user alice \
  --group staff \
  --budget 45m \
  --shift-timeout 3600 \
  --unit-timeout 4200 \
  --model opus \
  --on-calendar 'Mon..Fri 02:00' \
  --cron-schedule '17 * * * *'

python3 - "$output" "$scratch" <<'PY'
import pathlib
import plistlib
import sys

output = pathlib.Path(sys.argv[1])
scratch = sys.argv[2]
expected = {
    "com.letatech.mellions.runner.plist",
    "mellions-runner.crontab",
    "mellions-shift.service",
    "mellions-shift.timer",
}
assert {path.name for path in output.iterdir()} == expected

def normalized(name):
    value = (output / name).read_text().replace(scratch, "<TMP>")
    assert "@@" not in value
    return value

with (output / "com.letatech.mellions.runner.plist").open("rb") as handle:
    plist = plistlib.load(handle)
assert plist["ProgramArguments"] == [f"{scratch}/Acme & Co 50%/checkout/scripts/shifts.sh"]
assert plist["EnvironmentVariables"] == {
    "PATH": "/opt/acme & co/bin:/usr/bin:/bin",
    "MELLIONS_HOME": f"{scratch}/state and reports",
    "MELLIONS_WORKDIR": f"{scratch}/work area",
    "MELLIONS_CHECKOUT": f"{scratch}/Acme & Co 50%/checkout",
}
assert plist["WorkingDirectory"] == f"{scratch}/work area"
assert plist["KeepAlive"] == {"SuccessfulExit": False}
assert plist["StandardOutPath"] == f"{scratch}/state and reports/shifts/runner.out"
assert plist["StandardErrorPath"] == plist["StandardOutPath"]

cron = normalized("mellions-runner.crontab")
for literal in (
    "PATH='/opt/acme & co/bin:/usr/bin:/bin'",
    "CLAUDE_BIN=<TMP>/bin/claude",
    "MELLIONS_HOME='<TMP>/state and reports'",
    "MELLIONS_WORKDIR='<TMP>/work area'",
    "MELLIONS_CHECKOUT='<TMP>/Acme & Co 50%/checkout'",
    "@reboot '<TMP>/Acme & Co 50\\%/checkout/scripts/shifts.sh' >> '<TMP>/state and reports/shifts/runner.out' 2>&1",
    "17 * * * * '<TMP>/Acme & Co 50\\%/checkout/scripts/shifts.sh' >> '<TMP>/state and reports/shifts/runner.out' 2>&1",
):
    assert literal in cron, literal

service = normalized("mellions-shift.service")
for literal in (
    "User=alice",
    "Group=staff",
    'WorkingDirectory="<TMP>/work area"',
    'Environment="PATH=/opt/acme & co/bin:/usr/bin:/bin"',
    'Environment="HOME=<TMP>/user home"',
    'Environment="MELLIONS_HOME=<TMP>/state and reports"',
    'Environment="MELLIONS_WORKDIR=<TMP>/work area"',
    'Environment="MELLIONS_BUDGET=45m"',
    'Environment="MELLIONS_TIMEOUT=3600"',
    'Environment="MELLIONS_MODEL=opus"',
    'ExecStart="<TMP>/Acme & Co 50%%/checkout/scripts/shift.sh"',
    "TimeoutStartSec=4200",
):
    assert literal in service, literal

timer = normalized("mellions-shift.timer")
assert 'Documentation="file:<TMP>/Acme & Co 50%%/checkout/docs/architecture.md"' in timer
assert 'OnCalendar="Mon..Fri 02:00"' in timer
PY

if python3 "$here/deploy/render_schedulers.py" \
  --output-dir "$scratch/rejected" --checkout relative/path \
  --user-home "$user_home" --mellions-home "$mellions_home" \
  --workdir "$workdir" --path /usr/bin:/bin --claude-bin "$claude_bin" \
  --user alice --group staff --budget 45m --shift-timeout 3600 \
  --unit-timeout 4200 --model opus --on-calendar daily \
  --cron-schedule '0 * * * *' >/dev/null 2>&1; then
  echo "renderer accepted a relative checkout" >&2
  exit 1
fi

if python3 "$here/deploy/render_schedulers.py" \
  --output-dir "$scratch/rejected-schedule" --checkout "$checkout" \
  --user-home "$user_home" --mellions-home "$mellions_home" \
  --workdir "$workdir" --path /usr/bin:/bin --claude-bin "$claude_bin" \
  --user alice --group staff --budget 45m --shift-timeout 3600 \
  --unit-timeout 4200 --model opus --on-calendar daily \
  --cron-schedule '* * * * *;touch' >/dev/null 2>&1; then
  echo "renderer accepted a cron command suffix" >&2
  exit 1
fi

private_macos_user="lu""cas"
private_linux_user="le""ta"
private_pattern="/Users/${private_macos_user}|/home/${private_linux_user}|10\\.118\\."
planted="$scratch/private-control"
printf '/home/%s/private\n' "$private_linux_user" > "$planted"
grep -Eq "$private_pattern" "$planted" || {
  echo "private-path detector missed its planted control" >&2
  exit 1
}
if grep -ERn "$private_pattern" "$here/deploy"; then
  echo "private host path remains in the deployment surface" >&2
  exit 1
fi
printf '/home/you/project\n' > "$scratch/synthetic-control"
if grep -Eq "$private_pattern" "$scratch/synthetic-control"; then
  echo "private-path detector rejected the synthetic control" >&2
  exit 1
fi

echo "render scheduler tests passed"
