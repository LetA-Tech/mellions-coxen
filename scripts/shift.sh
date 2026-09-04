#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# One unattended shift: collect the estate's state, hand it to the engineer,
# file what comes back. Or, with MELLIONS_PROMPT naming a file, hand the
# engineer that task instead of the survey — the owner dispatching one piece
# of work to be carried while they are away.
#
# This script does no engineering and makes no decisions. Everything that looks
# like judgment — what to work on, how deep to verify, when to stop — belongs to
# the session.
#
# The unattended boundary is the runtime's own permission system, not this
# script's: the session runs with an explicit allow list and a deny list of the
# irreversible actions (deploy/unattended-settings.json), and a denied command
# is refused by the runtime whatever the session decides. The identity says the
# same thing in words; the settings file is what holds when the words do not.
set -uo pipefail

# The whole script is one function so bash parses all of it before running any
# of it. A shift runs for an hour; an edit to this file during that hour used
# to be read by the running copy at the old byte offsets, and the shift died at
# a syntax error in a line nobody had written.
main() {
here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MELLIONS="${MELLIONS_BIN:-$(command -v mellions || true)}"
CLAUDE="${CLAUDE_BIN:-$(command -v claude || true)}"
PYTHON="${MELLIONS_PYTHON:-$(command -v python3 || true)}"
MODEL="${MELLIONS_MODEL:-opus}"
BUDGET="${MELLIONS_BUDGET:-45m}"
SETTINGS="${MELLIONS_SETTINGS:-$here/deploy/unattended-settings.json}"
TASK="${MELLIONS_PROMPT:-}"

for need in "$MELLIONS" "$CLAUDE"; do
  [[ -x "$need" ]] || { echo "shift: not executable: ${need:-<unset>} (set MELLIONS_BIN / CLAUDE_BIN)" >&2; exit 2; }
done

# The home is the binary's, not this script's. `config home` is MELLIONS_HOME
# where it is set and the installation's report root otherwise, so an override
# still overrides and a host whose report_root is not under $HOME/mellions gets
# one home rather than two. A script that defaulted on its own wrote a second,
# configless home beside the real one: the shifts in it were invisible to
# `report digest` and to `doctor`, and the backstop below never saw the report
# the session had just written.
#
# The fallback is for an installation with no config at all, where the binary
# has no answer to give and the historical default is the only one there is.
# `config reports` is asked separately because a report is written under the
# report root whatever MELLIONS_HOME says, so $HOME_DIR/reports is the wrong
# place to look for one as soon as the two differ.
# Bounded, because these run before the log exists: a binary that hangs here —
# a config on a stalled mount — would hang the shift with nowhere to say so.
ASK=(); command -v timeout >/dev/null 2>&1 && ASK=(timeout 10)
HOME_DIR=$(${ASK[@]+"${ASK[@]}"} "$MELLIONS" config home 2>/dev/null | head -1)
REPORTS=$(${ASK[@]+"${ASK[@]}"} "$MELLIONS" config reports 2>/dev/null | head -1)
# Refused rather than guessed. There is no safe default here: a guess this
# script kept to itself splits its files from the report the session writes,
# and a guess exported to the session is silently wrong in every `mellions
# state` the awareness hook runs — the owner-away note among them. A binary
# that cannot say where its home is cannot file a report either, so a shift
# that continued would run its hour and have nowhere to put the result. It is
# refused here with python3 and the settings rather than discovered in the
# morning.
if [ -z "$HOME_DIR" ] || [ -z "$REPORTS" ]; then
  echo "shift: $MELLIONS cannot say where this installation's home is, so there is nowhere to put a shift. Run \`$MELLIONS config home\` and \`$MELLIONS config reports\` and read why: no config, a config that will not parse, or a binary older than those verbs — which is what an update that pulled and failed to install leaves behind." >&2
  exit 2
fi
# The session is handed the answer rather than resolving it again, so
# `report digest`, `away`/`back`, `doctor`'s runner line and the awareness
# hook's owner marker agree with this script even if the config changes
# underneath the shift. `assign`, `report write` and `who` do not read it —
# they resolve the config's roots directly, which is why the backstop asks for
# the reports directory rather than appending /reports here.
export MELLIONS_HOME="$HOME_DIR"
WORKDIR="${MELLIONS_WORKDIR:-$HOME_DIR}"
# Go puts a build's scratch in $GOTMPDIR, and unset that is /tmp — on a host
# where /tmp is a small tmpfs under a per-user quota, the shared resource every
# session on the machine needs. `go build` removes its work directory when it
# exits, and a shift is ended by things that give it no exit: a usage limit,
# the budget, SIGKILL. What is left behind is never collected, so the tmpfs
# fills with the scratch of builds that died, until a write of a few megabytes
# is refused with EDQUOT while df still reports gigabytes free and every suite
# on the host stops running.
#
# So a shift's scratch goes to disk under the home, where a leak costs disk
# rather than the tmpfs, and each shift removes what earlier ones left there.
# The sweep is bounded to this directory because it is the only one Mellions
# owns: scratch under /tmp belongs to whoever wrote it. Half a day is far
# longer than any shift, so nothing a live build holds is inside it.
#
# An explicit GOTMPDIR is honoured — whoever set it chose it. A directory that
# cannot be created leaves GOTMPDIR unset and Go on its own default, which is
# the degraded state this avoids rather than a broken one.
if [ -z "${GOTMPDIR:-}" ]; then
  scratch="$HOME_DIR/tmp/go"
  if mkdir -p "$scratch" 2>/dev/null; then
    find "$scratch" -maxdepth 1 -type d -name 'go-build*' -mmin +720 -exec rm -rf {} + 2>/dev/null
    export GOTMPDIR="$scratch"
  fi
fi
# python3 renders the stream and writes the reply. Without it the shift still
# runs a full session and then files it as having said nothing, so it is
# refused here with the rest rather than discovered in the morning.
[[ -x "$PYTHON" ]] || { echo "shift: no python3: ${PYTHON:-<unset>} (set MELLIONS_PYTHON) — it is what captures the session's reply, and without it a shift that worked is filed as having said nothing" >&2; exit 2; }
[[ -f "$SETTINGS" ]] || { echo "shift: no unattended settings at $SETTINGS — refusing to run without the deny list" >&2; exit 2; }

# The id names five files — log, survey, prompt, reply, stream — so it has to
# be unique per shift, not per second. Two launches in the same second once
# took the same id and one shift's hour was overwritten by the other's. The
# log is claimed with O_EXCL: whoever creates the file owns the id, and a shift
# that loses the race takes the next suffix instead of writing into the
# winner's files. The filesystem settles it, so no assumption about clocks,
# process ids or which host is writing has to hold.
mkdir -p "$HOME_DIR/shifts"
claim_id() {
  local base n=1 id
  base=$(date -u +%Y%m%d-%H%M%S)
  while [ "$n" -le 99 ]; do
    id=$base; [ "$n" -gt 1 ] && id="$base-$n"
    if (set -o noclobber; : > "$HOME_DIR/shifts/$id.log") 2>/dev/null; then
      printf '%s\n' "$id"; return 0
    fi
    n=$((n + 1))
  done
  # The loop suppressed every refusal, so repeat the last attempt with its
  # stderr shown: a full disk, a read-only mount and a genuinely taken id all
  # end up here, and the one line the operator gets should be the errno rather
  # than a guess between them.
  (set -o noclobber; : > "$HOME_DIR/shifts/$base-$((n - 1)).log")
  return 1
}
stamp=$(claim_id) || { echo "shift: cannot claim a shift id under $HOME_DIR/shifts — 99 attempts failed, the last one above" >&2; exit 2; }
log="$HOME_DIR/shifts/$stamp.log"
say() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" | tee -a "$log"; }

budget_seconds() {
  case "$1" in
    *h) echo $(( ${1%h} * 3600 )) ;;
    *m) echo $(( ${1%m} * 60 )) ;;
    *s) echo "${1%s}" ;;
    *)  echo $(( ${1:-45} * 60 )) ;;
  esac
}
deadline_epoch=$(( $(date -u +%s) + $(budget_seconds "$BUDGET") ))
# The session's hooks inherit this and print the clock and the time left on
# every tool call, so the deadline is read rather than estimated.
export MELLIONS_DEADLINE="$deadline_epoch"
deadline=$(date -u -d "@$deadline_epoch" +"%H:%M UTC" 2>/dev/null \
        || date -u -r "$deadline_epoch" +"%H:%M UTC" 2>/dev/null || echo "unknown")

TIMEOUT=()
if command -v timeout >/dev/null 2>&1; then TIMEOUT=(timeout "${MELLIONS_TIMEOUT:-3600}");
elif command -v gtimeout >/dev/null 2>&1; then TIMEOUT=(gtimeout "${MELLIONS_TIMEOUT:-3600}"); fi

say "shift $stamp starting (model=$MODEL budget=$BUDGET settings=$SETTINGS)"
[ "${#TIMEOUT[@]}" -eq 0 ] && say "no timeout(1) or gtimeout(1) on PATH — the session runs unbounded; MELLIONS_BUDGET still tells it when to stop, but nothing enforces it"

# ---- 1. situational awareness ------------------------------------------------
survey="$HOME_DIR/shifts/$stamp.survey.md"
if [ -n "$TASK" ]; then
  [ -f "$TASK" ] || { say "FATAL: MELLIONS_PROMPT=$TASK is not a file"; exit 2; }
  : > "$survey"
  say "task: $TASK ($(wc -c < "$TASK" | tr -d ' ') bytes); the survey is skipped"
# MELLIONS_SURVEY_ARGS is unquoted on purpose: it is extra words for the
# survey, such as `-repos mellions-coxen`, which is how the runner scopes
# every Nth shift to the engineer's own repository.
elif ! "$MELLIONS" survey -save ${MELLIONS_SURVEY_ARGS:-} > "$survey" 2>>"$log"; then
  say "survey incomplete — continuing; the survey says which sources did not answer"
  say "survey: $(wc -l < "$survey" | tr -d ' ') lines${MELLIONS_SURVEY_ARGS:+ ($MELLIONS_SURVEY_ARGS)}"
else
  say "survey: $(wc -l < "$survey" | tr -d ' ') lines${MELLIONS_SURVEY_ARGS:+ ($MELLIONS_SURVEY_ARGS)}"
fi

# ---- 2. the shift ------------------------------------------------------------
# Identity, partnership, program and work in flight reach the session through
# the plugin's session-start hook, exactly as they reach an interactive one --
# including the identity's instruction to load the Skill whose triggers the work
# matches. This prompt adds only what an unattended run needs: that nobody is
# reachable, the budget, the survey, and that dispatching subagents is asked
# for. The last of those is here rather than in a hook because the runtime's
# own instruction on this host is conditioned on the user having requested it,
# and the prompt is the user turn.
# What the last shift chose, passed over and left open, so this one does not
# start from nothing. The newest reply on disk is the previous shift's: this
# shift's own is written only when it ends. The tail, because the end of a
# reply is where a shift says what it left open. Not for a directed task —
# the owner said what the work is.
previous_reply() {
  local prev
  prev=$(ls -t "$HOME_DIR"/shifts/*.reply.md 2>/dev/null | head -1)
  [ -n "$prev" ] || return 0
  printf '\n=== THE PREVIOUS SHIFT SAID ===\n\n'
  if [ -s "$prev" ]; then
    printf 'Shift %s; the last 4 KB of its reply at most:\n\n' "$(basename "$prev" .reply.md)"
    tail -c 4096 "$prev"
    printf '\n'
  else
    printf 'Shift %s ended with no reply.\n' "$(basename "$prev" .reply.md)"
  fi
}

promptfile="$HOME_DIR/shifts/$stamp.prompt.md"
{
  cat <<EOF
You are running UNATTENDED: the owner is not reachable until morning.

Produce reversible artifacts — branches, commits, evidence, draft pull
requests, written decision packages. Git is your judgment, as is the scratch
you create. Merging a pull request whose claims you have established are ready
is yours, and so is starting a container the engineering work needs — torn down
in the same turn, with the footprint reported. What the partnership has not
delegated — deploying, migrating a shared or production database, a credential
operation, anything a person would want to be asked about — is left as a
decision package. The runtime denies a short
list outright (deleting a repository, secrets, privilege escalation, applying
or destroying infrastructure); a refusal there is the boundary working, never
an obstacle to route around. Stopping with a written decision package is a
successful outcome.

Subagents are yours to dispatch, and this paragraph is the owner asking for
them: \`Agent(*)\` is allowed in the settings this session runs under. Where a
method calls for one, dispatch it — the cold read \`mellions-self-learning\`
wants before a method changes, the independent reading \`mellions-reasoning\`
wants where being wrong is expensive, the breadth \`mellions-deep-research\`
says to delegate rather than read into this context. Some runtimes add a line
telling a session not to call the Agent tool unless the user requested it; the
user has, here. Fan-out for its own sake is still waste, and the verdict is
never delegated.

Your budget runs until $deadline. It is now $(date -u +'%H:%M UTC'). Every tool
result carries a \`clock:\` line with the time and the minutes left: read it
there rather than estimating, and \`date -u\` when in doubt. When the deadline
passes, write where the work stands rather than continuing silently or
abandoning it. Finishing early is fine and normal.

EOF
  if [ -n "$TASK" ]; then
    cat <<EOF
The owner has handed you the work below. Claim it with \`mellions assign open
... -because "the owner asked for it"\` and carry it through: reconstruct
current truth with \`mellions-deep-research\` (the code wins over an issue's
account of it), root-cause independently, implement, test, verify to the depth the claim requires, and
finish it — a draft pull request with the evidence in its body, a handoff on
the assignment, and \`mellions report write\` leading with anything that needs
the owner. Then ask what this should change about how the next work is done —
\`mellions-self-learning\` — and put that where it binds or say that nothing
earned it.

Do not ask questions you can answer by investigating. Unknown is not an
escalation, ambiguity is not an escalation, and a hard bug is not an escalation.

=== THE WORK ===

EOF
    cat "$TASK"
  else
    cat <<EOF
Read the engineering state below against the program you carry, choose one
piece of work, say what you considered and what you passed over, claim it with
\`mellions assign open ... -because "..." -not-chosen "..."\`, and carry it
through.

A peer's draft pull request left ready for review — evidence in its body, a
handoff on its lane — is engineering work available to you, and the independent
reading it wanted is exactly what a different session provides. Weigh the ones
under "Changes under review" alongside the open issues: an evidenced draft that
nobody has reviewed is finished work going undone, and unattended there is no
one else to read it. Never one of your own lanes. If you choose one, load
\`mellions-delegation\` before you open the diff and follow it there. This
paragraph says why such a draft is worth choosing, and which you may not; it
deliberately does not say how to review one, because a summary here reads as a
substitute for the Skill and is a lossier one than it looks.

Whatever you choose, carry it: reconstruct current truth with \`mellions-deep-research\` (the code wins over
an issue's account of it), root-cause independently, implement, test, verify to the depth the claim
requires, and finish it — a draft pull request with the evidence in its body,
a handoff on the assignment, and \`mellions report write\` leading with anything
that needs the owner. If nothing material happened, say so. Then ask what this
shift should change about how the next one works — \`mellions-self-learning\` —
and put that where it binds or say that nothing earned it.

Do not ask questions you can answer by investigating. Unknown is not an
escalation, ambiguity is not an escalation, and a hard bug is not an escalation.

=== ENGINEERING STATE ===

EOF
    cat "$survey"
    previous_reply
  fi
} > "$promptfile"

say "starting session"
reply="$HOME_DIR/shifts/$stamp.reply.md"
stream="$HOME_DIR/shifts/$stamp.stream.jsonl"

# Streamed rather than buffered, so a shift in progress is observable; one line
# per tool call and per paragraph is enough to see where it is.
#
# An empty TIMEOUT is expanded through `${TIMEOUT[@]+…}`: bash 3.2, which is
# what `/usr/bin/env bash` finds on a stock macOS, treats a plain
# `"${EMPTY[@]}"` under `set -u` as an unbound variable and kills the shift
# here, after the log and the prompt are already written.
#
# A stop — the runner's, or the scheduler's — arrives as SIGTERM and is passed
# on to the session. timeout(1) puts itself and the session in a process group
# of its own, so the group is signalled and the session gets it whether or not
# that timeout forwards signals (GNU's and uutils' both do; neither is relied
# on). Without timeout the session is a plain child, reached by pid. A trapped
# signal returns bash from `wait` at once, so the wait is re-entered until the
# session has actually ended.
session="" interrupted=""
forward() {
  interrupted=1
  [ -n "$session" ] || return 0
  kill -TERM -- "-$session" 2>/dev/null || kill -TERM "$session" 2>/dev/null
}
trap forward TERM INT HUP
cd "$WORKDIR" || exit 2
${TIMEOUT[@]+"${TIMEOUT[@]}"} "$CLAUDE" -p --model "$MODEL" \
  --settings "$SETTINGS" --permission-mode acceptEdits \
  --output-format stream-json --verbose \
  < "$promptfile" > "$stream" 2>>"$log" &
session=$!
# The follower reads the stream file itself and stops when the session process
# is gone, so nothing here needs GNU tail's --pid.
"$PYTHON" -u "$here/scripts/shift-follow.py" "$reply" "$stream" "$session" >> "$log" 2>&1 &
follower=$!
wait $session
rc=$?
while [ -n "$interrupted" ] && kill -0 "$session" 2>/dev/null; do
  interrupted=""
  wait $session 2>/dev/null
  rc=$?
done
wait $follower 2>/dev/null
[ -f "$reply" ] || : > "$reply"
say "session exited $rc, reply $(wc -c < "$reply" | tr -d ' ') bytes"

# ---- 3. the account's window, not the work -----------------------------------
# A usage-limit refusal is the account ending the shift: the runtime answers one
# sentence naming when the window reopens and exits non-zero, with the lane's
# work still in its worktree and the budget-expiry path — the one that writes
# where things stand — never reached. Filed as a complete shift it reads in the
# morning as a session that ran and had nothing to say, and the runner starts
# the next one into the same wall.
#
# Matched on the reply alone. The reply is written only from the stream's
# `result` event (scripts/shift-follow.py:74-76), and the refusal produces one:
# that is why the observed shift's reply is the 52-byte sentence and nothing
# else. Reading the stream's tail as well would buy no refusal this misses and
# would cost a real false positive — a session killed at MELLIONS_TIMEOUT also
# exits non-zero, and one working on this defect has the sentence in its last
# few KB. A refusal that somehow emitted no result event leaves the reply empty
# and is filed below as a session that said nothing, which is true.
#
# The reset time is taken only when the sentence names UTC, which the observed
# refusal does. A bare local time is the runtime naming a zone this script
# cannot resolve, so it is read as "interrupted, reset time not named" and the
# runner falls back to its own wait rather than inventing an offset.
#
# The sentence alone is not the signal. A session working on this very defect
# quotes it — into its reply, and into every tool result in its stream — and
# would file itself as refused. What the account's refusal has and the quotation
# does not is that the session did not get to choose how it ended: the refusal
# exits non-zero. Both, or neither.
limit_epoch=0 limit_at="" limit=""
[ "$rc" -eq 0 ] || limit=$(cat "$reply" 2>/dev/null | "$PYTHON" -c '
import calendar, re, sys, time
text = sys.stdin.read()
m = re.search(r"(?:session|usage)\s+limit[^\n]{0,200}?reset[a-z]*(?:\s+at)?\s+(\d{1,2})(?::(\d{2}))?\s*(am|pm)?", text, re.I)
if not m:
    sys.exit(1)
tail = text[m.start():m.end() + 24]
hour, minute = int(m.group(1)), int(m.group(2) or 0)
suffix = (m.group(3) or "").lower()
if suffix == "pm" and hour < 12:
    hour += 12
elif suffix == "am" and hour == 12:
    hour = 0
if hour > 23 or minute > 59 or not re.search(r"\bUTC\b", tail, re.I):
    print("0 -")
    sys.exit(0)
now = time.time()
u = time.gmtime(now)
target = calendar.timegm((u.tm_year, u.tm_mon, u.tm_mday, hour, minute, 0, 0, 0, 0))
if target <= now:
    target += 86400
print("%d %02d:%02d" % (target, hour, minute))
') || limit=""
if [ -n "$limit" ]; then
  limit_epoch=${limit%% *} limit_at=${limit#* }
  [ "$limit_at" = "-" ] && limit_at=""
  session_id=$(head -20 "$stream" 2>/dev/null | "$PYTHON" -c '
import json, sys
for line in sys.stdin:
    try:
        sid = json.loads(line).get("session_id")
    except ValueError:
        continue
    if sid:
        print(sid)
        break
' 2>/dev/null)
  resume=${session_id:+claude --resume $session_id}
  when=${limit_at:+, resumes after $limit_at UTC}
  say "shift $stamp interrupted by the usage limit${when:-, reset time not named}"
  # What the runner reads. Written before anything that can fail, so a shift
  # killed while filing still leaves the window on disk.
  # Three fields, never an empty one: the reader splits on whitespace, and a
  # blank middle field would hand it the session id as the reset time.
  printf '%s %s %s\n' "$limit_epoch" "${limit_at:--}" "${session_id:--}" > "$HOME_DIR/shifts/$stamp.limit"
  # The lane, not only the shift. An assignment record carries the session that
  # last worked it, so the lanes this shift cut are the ones naming this
  # session — marked here as interrupted rather than left looking like any
  # other active lane to the next `mellions continue`.
  if [ -n "$session_id" ]; then
    for record in "$HOME_DIR"/assignments/*/assignment.json; do
      [ -e "$record" ] || continue
      grep -q "$session_id" "$record" || continue
      lane=$(basename "$(dirname "$record")")
      "$MELLIONS" assign record "$lane" -kind next \
        "Shift $stamp was interrupted by the account's usage limit${when}, not by the work finishing or failing. Nothing here is a handoff: the worktree holds whatever the session had reached. ${resume:-The session id was not recorded in the stream.}" \
        >> "$log" 2>&1
      say "lane $lane marked interrupted"
    done
  fi
  "$MELLIONS" report write \
    -did "Shift $stamp was interrupted by the account's usage limit${when}. It did not finish and it did not fail: the runtime refused the session's next call. Any lane it opened is still active with its work in the worktree${resume:+ — $resume}." \
    -blocked "the account's usage window, not the work" >> "$log" 2>&1
  exit 3
fi

# ---- 4. file what came back --------------------------------------------------
if [ ! -s "$reply" ]; then
  say "the session said nothing — recording that rather than hiding it"
  "$MELLIONS" report write -did "Shift $stamp produced no reply (session exit $rc). Nothing was filed. Check $log." \
    -blocked "the session itself" >> "$log" 2>&1
  exit 1
fi
say "reply:"
sed 's/^/    /' "$reply" | tee -a "$log"

# The session files its own report when it has something to say; this is the
# backstop for a shift that worked and did not. `report write` claims its
# name with O_CREATE|O_EXCL before writing the content, so a claim a crash
# interrupted before the write landed is an empty .md — never a filed
# report — and `! -empty` is what keeps it from being read as one.
filed=$(find "$REPORTS" -newer "$survey" -name "*.md" ! -empty 2>/dev/null | head -1)
if [ -z "$filed" ]; then
  say "no report filed by the session; filing its reply as one"
  "$MELLIONS" report write -did "$(cat "$reply")" >> "$log" 2>&1
fi
say "shift $stamp complete"
}

main "$@"
