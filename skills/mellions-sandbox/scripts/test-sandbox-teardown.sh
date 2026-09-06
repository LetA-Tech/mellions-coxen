#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# `leta.sandbox=1` marks a container findable for teardown, not one this
# wrapper started -- repository harnesses apply it to their own test databases
# too. A teardown that removes every labelled container therefore destroys
# other lanes' running databases, and the guard that would have said so runs
# after the removal, where it protects nothing. Neither wrapper may remove a
# container it did not start; both are driven here against fake docker and
# colima so the case needs no container runtime.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d); tmp=$(cd "$tmp" && pwd -P)
trap 'rm -rf "$tmp"' EXIT INT TERM HUP
mkdir -p "$tmp/bin"

# A docker whose `ps` reports one foreign container -- another lane's test
# database, carrying the label because its harness applies it. Every call is
# appended to $tmp/calls, so a removal cannot happen without being recorded.
cat > "$tmp/bin/docker" <<'DOCKER'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$CALLS"
case "$1" in
  ps) [ -n "${SBX_EMPTY:-}" ] && exit 0
      case "$*" in
        *-aq*|*" -q"*) echo c0ffee00 ;;
        *) printf 'other-lane-pg\tpostgres:18\tUp 3 hours\n' ;;
      esac ;;
  info) case "$*" in *Runtimes*) echo '{"runsc":{"path":"runsc"}}' ;; *) echo 4 ;; esac ;;
  inspect) echo 0 ;;
esac
exit 0
DOCKER
cat > "$tmp/bin/colima" <<'COLIMA'
#!/usr/bin/env bash
printf 'colima %s\n' "$*" >> "$CALLS"
case "$1" in
  list) printf 'PROFILE STATUS\ndefault %s\n' "${COLIMA_STATE:-Running}" ;;
  stop) echo stopped >> "$CALLS" ;;
esac
exit 0
COLIMA
chmod +x "$tmp/bin/docker" "$tmp/bin/colima"

for sbx in leta-mac-sbx leta-linux-sbx; do
    CALLS="$tmp/calls-$sbx"; : > "$CALLS"
    out=$(CALLS="$CALLS" PATH="$tmp/bin:/usr/bin:/bin" bash "$root/$sbx" down 2>&1); rc=$?

    if grep -qE '(^| )rm( |$)' "$CALLS"; then
        bad "$sbx down invoked a removal: $(grep -E '(^| )rm( |$)' "$CALLS" | head -1)"
    else
        note "$sbx down removes nothing it did not start"
    fi
    if [ "$rc" -eq 0 ]; then
        bad "$sbx down exited 0 while a foreign container was up"
    else
        note "$sbx down refuses while a container survives"
    fi
    case "$out" in
        *other-lane-pg*) note "$sbx down names what survived" ;;
        *) bad "$sbx down did not name the surviving container: $out" ;;
    esac
done

# The macOS wrapper releases the VM only once nothing is left in it: a teardown
# that never reaches `colima stop` leaves the host's memory held.
CALLS="$tmp/calls-empty"; : > "$CALLS"
CALLS="$CALLS" SBX_EMPTY=1 PATH="$tmp/bin:/usr/bin:/bin" \
    bash "$root/leta-mac-sbx" down >/dev/null 2>&1
if grep -q '^colima stop' "$CALLS"; then
    note "leta-mac-sbx down stops the VM once the runtime is empty"
else
    bad "leta-mac-sbx down left the VM running with nothing in it"
fi

# Ownership has to be expressible or a reaper cannot be correct: the shared
# label says findable, never whose. Every run carries both.
for sbx in leta-mac-sbx leta-linux-sbx; do
    CALLS="$tmp/calls-run-$sbx"; : > "$CALLS"
    CALLS="$CALLS" LETA_SBX_OWNER=lane-under-test PATH="$tmp/bin:/usr/bin:/bin" \
        bash "$root/$sbx" -i alpine:3.22 -- true >/dev/null 2>&1
    run_call=$(grep '^run ' "$CALLS" | head -1)
    case "$run_call" in
        *"--label leta.sandbox=1"*) note "$sbx run is findable for teardown" ;;
        *) bad "$sbx run carries no leta.sandbox label: $run_call" ;;
    esac
    case "$run_call" in
        *"--label leta.sandbox.owner=lane-under-test"*) note "$sbx run says whose it is" ;;
        *) bad "$sbx run carries no owner label: $run_call" ;;
    esac
done

[ "$fail" -eq 0 ] && printf 'ok sandbox-teardown\n'
exit "$fail"
