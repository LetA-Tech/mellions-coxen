#!/usr/bin/env python3
"""Render host-specific scheduler files from the public templates."""

from __future__ import annotations

import argparse
import os
import re
import shlex
import tempfile
from pathlib import Path
from xml.sax.saxutils import escape as xml_escape


TOKEN = re.compile(r"@@[A-Z0-9_]+@@")
ACCOUNT = re.compile(r"[A-Za-z0-9_.-]+")
CRON_FIELD = re.compile(r"[A-Za-z0-9*/,-]+")


def text(value: str, label: str) -> str:
    if not value or any(ord(char) < 32 or ord(char) == 127 for char in value):
        raise ValueError(f"{label} must be non-empty and contain no control characters")
    return value


def absolute(value: str, label: str) -> str:
    value = os.path.expanduser(text(value, label))
    if not os.path.isabs(value):
        raise ValueError(f"{label} must be an absolute path")
    return os.path.normpath(value)


def account(value: str, label: str) -> str:
    value = text(value, label)
    if not ACCOUNT.fullmatch(value):
        raise ValueError(f"{label} may contain only letters, digits, dot, underscore, and hyphen")
    return value


def systemd_quote(value: str) -> str:
    value = text(value, "systemd value").replace("%", "%%")
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'


def cron_word(value: str) -> str:
    # cron treats an unescaped percent as a newline even inside shell quotes.
    return shlex.quote(text(value, "cron value")).replace("%", r"\%")


def cron_environment(value: str) -> str:
    return shlex.quote(text(value, "cron environment value"))


def render(template: Path, destination: Path, values: dict[str, str]) -> None:
    output = template.read_text()
    for token, value in values.items():
        output = output.replace(f"@@{token}@@", value)
    unresolved = sorted(set(TOKEN.findall(output)))
    if unresolved:
        raise ValueError(f"{template.name} has unresolved tokens: {', '.join(unresolved)}")

    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="w", dir=destination.parent, prefix=f".{destination.name}.", delete=False
    ) as handle:
        handle.write(output)
        temporary = Path(handle.name)
    temporary.chmod(0o644)
    temporary.replace(destination)


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(
        description="Render launchd, cron, and systemd scheduler files for one host."
    )
    result.add_argument("--output-dir", required=True)
    result.add_argument("--checkout", required=True)
    result.add_argument("--user-home", required=True)
    result.add_argument("--mellions-home", required=True)
    result.add_argument("--workdir", required=True)
    result.add_argument("--path", required=True)
    result.add_argument("--claude-bin", required=True)
    result.add_argument("--user", required=True)
    result.add_argument("--group", required=True)
    result.add_argument("--budget", required=True)
    result.add_argument("--shift-timeout", required=True, type=int)
    result.add_argument("--unit-timeout", required=True, type=int)
    result.add_argument("--model", required=True)
    result.add_argument("--on-calendar", required=True)
    result.add_argument("--cron-schedule", required=True)
    return result


def main() -> int:
    args = parser().parse_args()
    checkout = absolute(args.checkout, "--checkout")
    user_home = absolute(args.user_home, "--user-home")
    mellions_home = absolute(args.mellions_home, "--mellions-home")
    workdir = absolute(args.workdir, "--workdir")
    executable_path = text(args.path, "--path")
    claude_bin = absolute(args.claude_bin, "--claude-bin")
    user = account(args.user, "--user")
    group = account(args.group, "--group")
    budget = text(args.budget, "--budget")
    model = text(args.model, "--model")
    calendar = text(args.on_calendar, "--on-calendar")
    cron_schedule = text(args.cron_schedule, "--cron-schedule")
    cron_fields = cron_schedule.split()
    if len(cron_fields) != 5 or not all(CRON_FIELD.fullmatch(field) for field in cron_fields):
        raise ValueError("--cron-schedule must contain five portable cron fields")
    if args.shift_timeout <= 0 or args.unit_timeout < args.shift_timeout:
        raise ValueError("timeouts must be positive and --unit-timeout must cover --shift-timeout")

    runner = os.path.join(checkout, "scripts", "shifts.sh")
    shift = os.path.join(checkout, "scripts", "shift.sh")
    architecture = os.path.join(checkout, "docs", "architecture.md")
    runner_log = os.path.join(mellions_home, "shifts", "runner.out")
    templates = Path(__file__).resolve().parent / "templates"
    destination = Path(args.output_dir).expanduser().resolve()

    common_xml = {
        "RUNNER_XML": xml_escape(runner),
        "PATH_XML": xml_escape(executable_path),
        "MELLIONS_HOME_XML": xml_escape(mellions_home),
        "WORKDIR_XML": xml_escape(workdir),
        "CHECKOUT_XML": xml_escape(checkout),
        "RUNNER_LOG_XML": xml_escape(runner_log),
    }
    render(
        templates / "com.letatech.mellions.runner.plist.in",
        destination / "com.letatech.mellions.runner.plist",
        common_xml,
    )

    common_cron = {
        "PATH_CRON": cron_environment(executable_path),
        "CLAUDE_BIN_CRON": cron_environment(claude_bin),
        "MELLIONS_HOME_CRON": cron_environment(mellions_home),
        "WORKDIR_CRON": cron_environment(workdir),
        "CHECKOUT_CRON": cron_environment(checkout),
        "CRON_SCHEDULE": cron_schedule,
        "RUNNER_CRON": cron_word(runner),
        "RUNNER_LOG_CRON": cron_word(runner_log),
    }
    render(
        templates / "mellions-runner.crontab.in",
        destination / "mellions-runner.crontab",
        common_cron,
    )

    service_values = {
        "USER": user,
        "GROUP": group,
        "WORKDIR_SYSTEMD": systemd_quote(workdir),
        "PATH_SYSTEMD": systemd_quote(f"PATH={executable_path}"),
        "HOME_SYSTEMD": systemd_quote(f"HOME={user_home}"),
        "MELLIONS_HOME_SYSTEMD": systemd_quote(f"MELLIONS_HOME={mellions_home}"),
        "MELLIONS_WORKDIR_SYSTEMD": systemd_quote(f"MELLIONS_WORKDIR={workdir}"),
        "BUDGET_SYSTEMD": systemd_quote(f"MELLIONS_BUDGET={budget}"),
        "SHIFT_TIMEOUT_SYSTEMD": systemd_quote(f"MELLIONS_TIMEOUT={args.shift_timeout}"),
        "MODEL_SYSTEMD": systemd_quote(f"MELLIONS_MODEL={model}"),
        "SHIFT_SYSTEMD": systemd_quote(shift),
        "UNIT_TIMEOUT": str(args.unit_timeout),
    }
    render(
        templates / "mellions-shift.service.in",
        destination / "mellions-shift.service",
        service_values,
    )
    render(
        templates / "mellions-shift.timer.in",
        destination / "mellions-shift.timer",
        {
            "ARCHITECTURE_SYSTEMD": systemd_quote(f"file:{architecture}"),
            "CALENDAR_SYSTEMD": systemd_quote(calendar),
        },
    )
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except ValueError as error:
        raise SystemExit(f"render_schedulers: {error}") from error
