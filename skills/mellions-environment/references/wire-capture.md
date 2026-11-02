# Establishing what a session actually receives

A session cannot report what reached it, only what it believes. The request
on the wire settles it: `scripts/capture-wire.sh`, in the mellions-coxen
checkout, runs one headless session against a loopback server that records
the request body and answers 400 — nothing forwarded, no credential written —
and prints model, system-prompt size and tool count.

The tool array is often the decisive half: a tool absent from it was never offered, which a session reports as a
prohibition. Capture again with one input changed — model, settings file,
plugin set — when the question is why two hosts differ, not whether a line
exists.

The 400 stops the session at its first turn, so hook output and anything
injected later appear in no capture, and a subagent's prompt is not a
top-level `claude -p`'s: absence in a capture is absence from turn one.
