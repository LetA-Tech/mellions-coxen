#!/usr/bin/env python3
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
"""Render a Claude stream-json file as readable progress, and capture the reply.

A shift runs for up to an hour. With buffered output, a session that is working
and one that is wedged look identical for that whole time, which makes the only
available response "wait longer". One line per tool call and per paragraph is
enough to see where it is.

Raw tool RESULTS are deliberately dropped: they are the firehose that makes an
observation surface unreadable, and anything worth keeping from them ends up in
the assignment or the report.

The stream file is followed here rather than by `tail -F --pid=`: `--pid` is a
GNU coreutils option that macOS `tail` rejects, and a follower that never runs
captures no reply, which the shift then files as a session that said nothing.
Following the file needs only the standard library, so this depends on nothing
a stock macOS lacks.

Usage: shift-follow.py <reply-path> <stream-path> [session-pid]

With a session pid the follower also stops once that process is gone and the
stream holds no further complete line — the termination `--pid` provided.
Without one it stops only on the session's result event.
"""
import errno
import json
import os
import sys
import time

reply_path = sys.argv[1]
stream_path = sys.argv[2]
session_pid = int(sys.argv[3]) if len(sys.argv) > 3 else 0

POLL_SECONDS = 0.1


def session_alive():
    """Whether the session process still exists. A zombie counts as alive: the
    shift's own `wait` reaps it, and until then the stream may still hold lines
    this follower has not read."""
    if not session_pid:
        return True
    try:
        os.kill(session_pid, 0)
    except OSError as exc:
        # EPERM means the process exists and is not ours to signal.
        return exc.errno == errno.EPERM
    return True


def handle(line):
    """Render one stream event. True when the session has finished."""
    line = line.strip()
    if not line:
        return False
    try:
        ev = json.loads(line)
    except ValueError:
        return False  # an unmodelled line is not a reason to stop watching

    kind = ev.get("type")
    if kind == "assistant":
        for block in ev.get("message", {}).get("content", []):
            if block.get("type") == "tool_use":
                got = block.get("input", {}) or {}
                arg = got.get("command") or got.get("file_path") or got.get("pattern") or ""
                print("  . {} {}".format(block.get("name"), str(arg)[:110]), flush=True)
            elif block.get("type") == "text":
                text = (block.get("text") or "").strip()
                if text:
                    print("  " + text.splitlines()[0][:160], flush=True)
    elif kind == "result":
        with open(reply_path, "w") as fh:
            fh.write(ev.get("result", "") or "")
        print("  [session finished]", flush=True)
        return True
    return False


def follow():
    stream = None
    try:
        while True:
            # Read once more on the pass that finds the session gone rather
            # than returning on it: everything the session wrote is on disk by
            # then, and a line written just before it exited is still owed.
            last_pass = not session_alive()

            if stream is None:
                try:
                    stream = open(stream_path)
                except OSError:
                    if last_pass:
                        return
                    time.sleep(POLL_SECONDS)
                    continue

            pos = stream.tell()
            line = stream.readline()
            if line.endswith("\n"):
                if handle(line):
                    return
                continue
            # A partial line means the writer is mid-record: rewind and wait
            # for the rest rather than parsing half of it.
            stream.seek(pos)
            if last_pass:
                return
            time.sleep(POLL_SECONDS)
    finally:
        if stream is not None:
            stream.close()


follow()
