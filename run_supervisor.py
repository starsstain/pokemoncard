#!/usr/bin/env python3
"""Start and supervise the PokemonCard frontend, attack, and battle services.

The script opens one console per service, checks whether each terminal process is
still running every three seconds, restarts closed terminals, and emits Datadog metrics.
"""

from __future__ import annotations

import argparse
import json
import os
import platform
import signal
import socket
import subprocess
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent
BACKEND = ROOT / "backend"
STATE_FILE = ROOT / ".pokemon_supervisor_state.json"
CHECK_INTERVAL_SECONDS = 3
RECURRENCE_WINDOW_SECONDS = 60 * 60
WARNING_DURATION_SECONDS = 60 * 60
DATADOG_STATIC_TAGS = (
    "env:school",
    "service:pokemoncard",
    "version:1.0.0",
    "team:bj_xsolla-school-group-dongxue",
)

def load_env_file() -> None:
    """Load root .env without adding a third-party dependency.

    Existing process environment variables take precedence over local settings.
    """
    env_file = ROOT / ".env"
    try:
        lines = env_file.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError:
        return
    except OSError as error:
        print(f"[supervisor] could not read {env_file}: {error}", file=sys.stderr)
        return

    for line_number, raw_line in enumerate(lines, start=1):
        line = raw_line.lstrip("\ufeff").strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[7:].lstrip()
        if "=" not in line:
            print(f"[supervisor] ignoring invalid .env line {line_number}", file=sys.stderr)
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        if key:
            os.environ.setdefault(key, value)


@dataclass
class Service:
    name: str
    command: str
    cwd: Path
    process: subprocess.Popen[bytes] | None = None
    started_at: float = field(default=0.0)


SERVICES = (
    Service("frontend", "npm run dev -- --host 127.0.0.1", ROOT),
    Service("attack", "go run ./attack", BACKEND),
    Service("battle", "go run ./battle", BACKEND),
)


def load_state() -> tuple[dict[str, float], dict[str, float]]:
    """Load failure timestamps and active one-hour warning windows."""
    try:
        raw_state = json.loads(STATE_FILE.read_text(encoding="utf-8"))
        last_failure_at = {
            str(name): float(value)
            for name, value in raw_state.get("last_failure_at", {}).items()
        }
        warning_until = {
            str(name): float(value)
            for name, value in raw_state.get("warning_until", {}).items()
        }
        return last_failure_at, warning_until
    except (FileNotFoundError, ValueError, OSError, TypeError):
        return {}, {}


def save_state(last_failure_at: dict[str, float], warning_until: dict[str, float]) -> None:
    temporary_file = STATE_FILE.with_suffix(".tmp")
    temporary_file.write_text(
        json.dumps(
            {"last_failure_at": last_failure_at, "warning_until": warning_until},
            indent=2,
        ),
        encoding="utf-8",
    )
    temporary_file.replace(STATE_FILE)

def send_failure_level(service: Service, value: int) -> None:
    """Emit a 0 (normal), 1 (warning), or 2 (error) metric value."""
    if value not in (0, 1, 2):
        raise ValueError("failure_level must be 0, 1, or 2")
    metric = (
        f"pokemoncard.supervisor.failure_level:{value}|g"
        f"|#{','.join((*DATADOG_STATIC_TAGS, f'component:{service.name}', 'source:local-supervisor'))}"
    )
    agent_host = os.environ.get("DATADOG_AGENT_HOST", "127.0.0.1")
    try:
        agent_port = int(os.environ.get("DD_DOGSTATSD_PORT", "8125"))
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as client:
            client.sendto(metric.encode("utf-8"), (agent_host, agent_port))
    except (OSError, ValueError) as error:
        print(f"[Datadog] could not send failure_level metric: {error}", file=sys.stderr)

def start_service(service: Service) -> None:
    print(f"[{service.name}] starting: {service.command}")
    if platform.system() == "Windows":
        # cmd exits with the service, so the separate console closes on a crash.
        service.process = subprocess.Popen(
            ["cmd.exe", "/d", "/s", "/c", f"title PokemonCard - {service.name} && {service.command}"],
            cwd=service.cwd,
            creationflags=subprocess.CREATE_NEW_CONSOLE,
        )
    else:
        service.process = subprocess.Popen(service.command, cwd=service.cwd, shell=True, start_new_session=True)
    service.started_at = time.monotonic()


def stop_service(service: Service) -> None:
    if service.process is None or service.process.poll() is not None:
        return
    try:
        if platform.system() == "Windows":
            subprocess.run(["taskkill", "/PID", str(service.process.pid), "/T", "/F"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
        else:
            os.killpg(os.getpgid(service.process.pid), signal.SIGTERM)
        service.process.wait(timeout=5)
    except (OSError, subprocess.TimeoutExpired):
        pass


def unhealthy_reason(service: Service) -> str | None:
    """Only inspect the terminal process; do not call service HTTP endpoints."""
    if service.process is None:
        return "the supervisor has no terminal process for this service"
    return_code = service.process.poll()
    if return_code is None:
        return None
    return f"the terminal process exited with code {return_code}"

def format_now() -> str:
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def supervise() -> int:
    last_failure_at, warning_until = load_state()
    restart_disabled: set[str] = set()
    for service in SERVICES:
        start_service(service)
    try:
        while True:
            time.sleep(CHECK_INTERVAL_SECONDS)
            for service in SERVICES:
                if service.name in restart_disabled:
                    send_failure_level(service, 2)
                    continue

                current_time = time.time()
                reason = unhealthy_reason(service)
                if reason is None:
                    if current_time < warning_until.get(service.name, 0):
                        send_failure_level(service, 1)
                    else:
                        if service.name in warning_until:
                            del warning_until[service.name]
                            save_state(last_failure_at, warning_until)
                        send_failure_level(service, 0)
                    continue

                previous_failure = last_failure_at.get(service.name)
                repeated_within_window = (
                    previous_failure is not None
                    and current_time - previous_failure <= RECURRENCE_WINDOW_SECONDS
                )
                level = "error" if repeated_within_window else "warning"
                print(f"[{format_now()}] [{level.upper()}] {service.name}: {reason}", file=sys.stderr)
                if repeated_within_window:
                    warning_until.pop(service.name, None)
                    send_failure_level(service, 2)
                else:
                    warning_until[service.name] = current_time + WARNING_DURATION_SECONDS
                    send_failure_level(service, 1)
                last_failure_at[service.name] = current_time
                save_state(last_failure_at, warning_until)
                stop_service(service)
                if repeated_within_window:
                    restart_disabled.add(service.name)
                    print(
                        f"[{service.name}] automatic restart disabled after two failures within one hour.",
                        file=sys.stderr,
                    )
                    continue
                start_service(service)
    except KeyboardInterrupt:
        print("\nStopping supervised services...")
        return 0
    finally:
        for service in SERVICES:
            stop_service(service)

def main() -> int:
    load_env_file()
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    return supervise()


if __name__ == "__main__":
    raise SystemExit(main())
