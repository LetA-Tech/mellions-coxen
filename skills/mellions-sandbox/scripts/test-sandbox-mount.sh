#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Colima shares a fixed set of host directories with its VM. A bind of a path
# outside them does not fail -- it mounts EMPTY -- so the run fails for missing
# sources and reads as a broken tree rather than a broken mount, which is a
# blocker that is not there. Measured on this host: /private/tmp mounts 0
# entries while the same tree under $HOME mounts its contents. The wrapper
# refuses the unshared path; this holds it to that against a fake docker.
set -uo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fail=0
note() { printf '  %s\n' "$*"; }
bad() { printf 'FAIL %s\n' "$*"; fail=1; }

tmp=$(mktemp -d); tmp=$(cd "$tmp" && pwd -P)
trap 'rm -rf "$tmp" "$shared_dir"' EXIT INT TERM HUP
mkdir -p "$tmp/bin"
cat > "$tmp/bin/docker" <<'DOCKER'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$CALLS"
case "$1" in
  ps) exit 0 ;;
  info) case "$*" in *Runtimes*) echo '{"runsc":{"path":"runsc"}}' ;; *) echo 4 ;; esac ;;
esac
exit 0
DOCKER
cat > "$tmp/bin/colima" <<'COLIMA'
#!/usr/bin/env bash
case "$1" in list) printf 'PROFILE STATUS\ndefault Running\n' ;; esac
exit 0
COLIMA
chmod +x "$tmp/bin/docker" "$tmp/bin/colima"

# An unshared root. mktemp -d lands under /var -> /private/var on macOS and
# under /tmp on Linux; neither is shared, which is the case under test.
unshared="$tmp/tree"; mkdir -p "$unshared"
shared_dir="$HOME/.cache/leta-sbx-mount-case-$$"; mkdir -p "$shared_dir"

CALLS="$tmp/calls-unshared"; : > "$CALLS"
out=$(CALLS="$CALLS" PATH="$tmp/bin:/usr/bin:/bin" bash "$root/leta-mac-sbx" -r "$unshared" -- true 2>&1); rc=$?
if [ "$rc" -eq 0 ]; then
    bad "an unshared mount path was accepted; it would have mounted empty"
elif ! printf '%s' "$out" | grep -q "does not share"; then
    bad "the refusal does not say the path is unshared: $out"
elif ! printf '%s' "$out" | grep -q 'HOME'; then
    bad "the refusal names no countermeasure: $out"
else
    note "an unshared mount path is refused, with the reason and the way out"
fi
if grep -q '^run ' "$CALLS"; then
    bad "docker run was reached despite the unshared path"
else
    note "no container is started on an unshared path"
fi

CALLS="$tmp/calls-shared"; : > "$CALLS"
CALLS="$CALLS" PATH="$tmp/bin:/usr/bin:/bin" bash "$root/leta-mac-sbx" -r "$shared_dir" -- true >/dev/null 2>&1
if grep -q '^run ' "$CALLS"; then
    note "a shared path under \$HOME still runs"
else
    bad "a shared path under \$HOME was refused; the guard matches everything"
fi

[ "$fail" -eq 0 ] && printf 'ok sandbox-mount\n'
exit "$fail"
