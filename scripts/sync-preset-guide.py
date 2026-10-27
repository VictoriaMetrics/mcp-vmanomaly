#!/usr/bin/env python3
"""Copy the canonical skills guide into the MCP distribution; no network access."""
import argparse
import hashlib
import json
import subprocess
from pathlib import Path

from resource_snapshot import publish_snapshot

SOURCE = Path("plugins/vmanomaly/skills/vmanomaly-preset/references/copilot-preset-guide-v1.md")
DESTINATION = Path(__file__).resolve().parents[1] / "internal/resources/guides"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("skills_checkout", type=Path)
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--allow-unmerged", action="store_true", help="Explicitly prepare a review snapshot before the skills PR merges")
    args = parser.parse_args()
    source = args.skills_checkout.resolve()
    remote = subprocess.check_output(["git", "-C", str(source), "remote", "get-url", "origin"], text=True).strip()
    canonical_remotes = {
        "https://github.com/VictoriaMetrics/skills", "https://github.com/VictoriaMetrics/skills.git",
        "git@github.com:VictoriaMetrics/skills.git", "ssh://git@github.com/VictoriaMetrics/skills.git",
    }
    if remote not in canonical_remotes:
        raise SystemExit("Expected origin to identify VictoriaMetrics/skills; use a canonical checkout.")
    merged = subprocess.run(["git", "-C", str(source), "merge-base", "--is-ancestor", "HEAD", "origin/main"],
                            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0
    if not merged and not args.allow_unmerged:
        raise SystemExit("Fetch skills origin/main and sync a merged revision, or use --allow-unmerged for review only.")
    content = (source / SOURCE).read_bytes()
    revision = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    committed = subprocess.check_output(["git", "-C", str(source), "show", f"HEAD:{SOURCE.as_posix()}"])
    if content != committed:
        raise SystemExit("Commit canonical guide changes before syncing.")
    metadata = {
        "version": 1,
        "source_status": "merged" if merged else "unmerged-review",
        "source_repository": "https://github.com/VictoriaMetrics/skills",
        "source_path": SOURCE.as_posix(),
        "source_revision": revision,
        "sha256": hashlib.sha256(content).hexdigest(),
    }
    outputs = {"preset-v1.md": content, "preset-v1.json": (json.dumps(metadata, indent=2) + "\n").encode()}
    if args.check:
        if not DESTINATION.exists() or {p.name for p in DESTINATION.iterdir()} != set(outputs):
            raise SystemExit("Guide snapshot has missing or unexpected files")
        for name, data in outputs.items():
            path = DESTINATION / name
            if not path.exists() or path.read_bytes() != data:
                raise SystemExit(f"Guide snapshot is stale: {path.name}")
        return

    def populate(staged: Path) -> None:
        for name, data in outputs.items():
            (staged / name).write_bytes(data)

    publish_snapshot(DESTINATION, populate)


if __name__ == "__main__":
    main()
