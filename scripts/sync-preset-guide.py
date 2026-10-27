#!/usr/bin/env python3
"""Copy the canonical skills guide into the MCP distribution; no network access."""
import argparse
import hashlib
import json
import shutil
import subprocess
from pathlib import Path

from resource_snapshot import publish_snapshot

SOURCE = Path("plugins/vmanomaly/skills/vmanomaly-preset/references/copilot-preset-guide-v1.md")
DESTINATION = Path(__file__).resolve().parents[1] / "internal/resources/guides"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("skills_checkout", type=Path)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    source = args.skills_checkout.resolve()
    content = (source / SOURCE).read_bytes()
    revision = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    committed = subprocess.check_output(["git", "-C", str(source), "show", f"HEAD:{SOURCE.as_posix()}"])
    if content != committed:
        raise SystemExit("Commit canonical guide changes before syncing.")
    metadata = {
        "version": 1,
        "source_repository": "https://github.com/VictoriaMetrics/skills",
        "source_path": SOURCE.as_posix(),
        "source_revision": revision,
        "sha256": hashlib.sha256(content).hexdigest(),
    }
    outputs = {"preset-v1.md": content, "preset-v1.json": (json.dumps(metadata, indent=2) + "\n").encode()}
    if args.check:
        for name, data in outputs.items():
            path = DESTINATION / name
            if not path.exists() or path.read_bytes() != data:
                raise SystemExit(f"Guide snapshot is stale: {path.name}")
        return

    def populate(staged: Path) -> None:
        if DESTINATION.exists():
            shutil.copytree(DESTINATION, staged, dirs_exist_ok=True)
        for name, data in outputs.items():
            (staged / name).write_bytes(data)

    publish_snapshot(DESTINATION, populate)


if __name__ == "__main__":
    main()
