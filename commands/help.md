---
description: What to type and what Mellions runs on its own — the slash commands, the CLI, the hooks
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

The person asked what they can type and what happens on its own. Answer with
the three parts below in this order, then stop; do not paraphrase the command
output, show it.

**1. Three kinds of command.** Print this, verbatim:

```
Three kinds of command, and who types them

  /mellions:<name>   a slash command of this plugin — you type it in the prompt; the
                     session does the rest. Type "/mellions" and the prompt completes
                     the list; "/help" lists every plugin's commands too.
  mellions <verb>    the CLI. The session runs it itself through Bash whenever the
                     work needs it — surveying, claiming a lane, recording, handing
                     off. You run it from a terminal when you want to see for
                     yourself: "mellions help" is the whole surface.
  hooks              automatic. At session start, on every prompt, on every tool call
                     and before compaction, the plugin's hooks run the CLI and hand the
                     session its identity, partnership, program, work in flight, peers,
                     the clock and what needs you. You never run one.

  You type: a requirement, a question, or a slash command. Mellions handles: finding
  and claiming work, investigating, implementing, verifying, the pull request, the
  handoff, the report, and what it learned.
```

**2. The slash commands of this plugin.** List them from the plugin's own
`commands/` directory, one line each — the file name without `.md` as
`/mellions:<name>`, then the file's `description` — so the list is what is
installed, never a copy: `ls "${CLAUDE_PLUGIN_ROOT}/commands"`. If that
variable is empty here, the directory is the load path `mellions doctor`
prints, plus `/commands`.

**3. The CLI.** Run `mellions help` and show its output whole.
