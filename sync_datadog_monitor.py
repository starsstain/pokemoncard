#!/usr/bin/env python3
"""Create or update PokemonCard Datadog monitors from JSON definitions."""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parent
MONITORS_DIRECTORY = ROOT / "monitors"
REQUIRED_FIELDS = ("name", "type", "query", "message", "options")


def load_env_file(path: Path) -> None:
    """Load local .env values without replacing variables set by the shell."""
    if not path.exists():
        return
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.lstrip("\ufeff").strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        if key:
            os.environ.setdefault(key, value)


def api_request(method: str, url: str, headers: dict[str, str], payload: dict[str, Any] | None = None) -> Any:
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    request = Request(url, data=data, headers=headers, method=method)
    try:
        with urlopen(request, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except HTTPError as error:
        details = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"Datadog API returned HTTP {error.code}: {details}") from error
    except URLError as error:
        raise RuntimeError(f"Could not reach the Datadog API: {error.reason}") from error


_MISSING = object()


def configuration_differences(desired: Any, actual: Any, path: str = "") -> list[tuple[str, Any, Any]]:
    """Compare only fields owned by the local JSON definition."""
    if isinstance(desired, dict):
        actual_mapping = actual if isinstance(actual, dict) else {}
        differences: list[tuple[str, Any, Any]] = []
        for key, value in desired.items():
            child_path = f"{path}.{key}" if path else key
            remote_value = actual_mapping.get(key, _MISSING)
            differences.extend(configuration_differences(value, remote_value, child_path))
        return differences
    if path == "tags" and isinstance(desired, list) and isinstance(actual, list):
        if sorted(desired) == sorted(actual):
            return []
    if desired == actual:
        return []
    return [(path, actual, desired)]


def display_value(value: Any) -> str:
    if value is _MISSING:
        return "<not set>"
    return json.dumps(value, ensure_ascii=False)


def print_plan(target_id: Any, definition_path: Path, definition: dict[str, Any], existing: dict[str, Any]) -> None:
    differences = configuration_differences(definition, existing)
    print(f"Would update Datadog monitor {target_id}: {definition['name']} ({definition_path.name})")
    if not differences:
        print("No differences found in fields managed by the local JSON file.")
        return
    print("Planned changes (Datadog -> local JSON):")
    for path, remote_value, local_value in differences:
        print(f"- {path}: {display_value(remote_value)} -> {display_value(local_value)}")


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create or update Datadog monitors from JSON definitions.")
    parser.add_argument(
        "--definition",
        type=Path,
        action="append",
        help="A monitor definition to sync. Repeat for multiple files. Defaults to every monitors/*.json file.",
    )
    parser.add_argument(
        "--monitor-id",
        help="Datadog monitor ID. Only valid when exactly one --definition is selected. Defaults to DD_MONITOR_ID in single-file mode.",
    )
    parser.add_argument("--site", help="Datadog site, for example datadoghq.com. Defaults to DD_SITE or datadoghq.com.")
    parser.add_argument("--dry-run", action="store_true", help="Show whether monitors would be created or updated.")
    return parser.parse_args()


def load_definition(path: Path) -> dict[str, Any]:
    try:
        definition = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as error:
        raise RuntimeError(f"Monitor definition not found: {path}") from error
    except json.JSONDecodeError as error:
        raise RuntimeError(f"Invalid JSON in {path}: {error}") from error
    if not isinstance(definition, dict):
        raise RuntimeError(f"Monitor definition must be a JSON object: {path}")
    for field in REQUIRED_FIELDS:
        if field not in definition:
            raise RuntimeError(f"The monitor definition {path} must include '{field}'.")
    return definition


def selected_definition_paths(arguments: argparse.Namespace) -> list[Path]:
    if arguments.definition:
        paths = [path.resolve() for path in arguments.definition]
    else:
        paths = sorted(MONITORS_DIRECTORY.glob("*.json"))
    if not paths:
        raise RuntimeError(f"No monitor definition files found in {MONITORS_DIRECTORY}.")
    if len(set(paths)) != len(paths):
        raise RuntimeError("The same monitor definition was supplied more than once.")
    return paths


def existing_monitor_for_definition(definition_path: Path, definition: dict[str, Any], monitors: list[dict[str, Any]]) -> dict[str, Any] | None:
    matches = [
        monitor
        for monitor in monitors
        if monitor.get("name") == definition["name"] and monitor.get("type") == definition["type"]
    ]
    if len(matches) > 1:
        raise RuntimeError(
            f"Found {len(matches)} monitors named '{definition['name']}' for {definition_path.name}. "
            "Use a single --definition with --monitor-id to select the intended monitor."
        )
    return matches[0] if matches else None


def main() -> int:
    arguments = parse_arguments()
    load_env_file(ROOT / ".env")

    definition_paths = selected_definition_paths(arguments)
    definitions = [(path, load_definition(path)) for path in definition_paths]
    if arguments.monitor_id and len(definitions) != 1:
        raise RuntimeError("--monitor-id can only be used when syncing exactly one monitor definition.")

    api_key = os.environ.get("DD_API_KEY")
    app_key = os.environ.get("DD_APP_KEY")
    if not api_key or not app_key:
        raise RuntimeError("DD_API_KEY and DD_APP_KEY are required. Add them to the local .env file or set them in this terminal.")

    site = (arguments.site or os.environ.get("DD_SITE") or "datadoghq.com").strip()
    site = site.removeprefix("https://api.").removeprefix("http://api.").removeprefix("api.").rstrip("/")
    headers = {
        "DD-API-KEY": api_key,
        "DD-APPLICATION-KEY": app_key,
        "Accept": "application/json",
        "Content-Type": "application/json",
    }
    api_base = f"https://api.{site}/api/v1/monitor"

    configured_monitor_id = os.environ.get("DD_MONITOR_ID") if len(definitions) == 1 else None
    monitor_id = arguments.monitor_id or configured_monitor_id
    targets: list[tuple[Path, dict[str, Any], dict[str, Any] | None]] = []

    if monitor_id:
        definition_path, definition = definitions[0]
        try:
            existing = api_request("GET", f"{api_base}/{monitor_id}", headers)
        except RuntimeError as error:
            raise RuntimeError(f"Could not read monitor ID {monitor_id}. Check DD_SITE, permissions, and the ID.\n{error}") from error
        targets.append((definition_path, definition, existing))
    else:
        summaries = api_request("GET", api_base, headers)
        for definition_path, definition in definitions:
            summary = existing_monitor_for_definition(definition_path, definition, summaries)
            existing = api_request("GET", f"{api_base}/{summary['id']}", headers) if summary else None
            targets.append((definition_path, definition, existing))

    for definition_path, definition, existing in targets:
        if existing:
            target_id = existing["id"]
            if arguments.dry_run:
                print_plan(target_id, definition_path, definition, existing)
            else:
                result = api_request("PUT", f"{api_base}/{target_id}", headers, definition)
                print(f"Updated Datadog monitor {result['id']}: {result['name']} ({definition_path.name})")
        elif arguments.dry_run:
            print(f"Would create Datadog monitor: {definition['name']} ({definition_path.name})")
        else:
            result = api_request("POST", api_base, headers, definition)
            print(f"Created Datadog monitor {result['id']}: {result['name']} ({definition_path.name})")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except RuntimeError as error:
        print(f"Error: {error}", file=sys.stderr)
        raise SystemExit(1)