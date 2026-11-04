#!/usr/bin/env python3
"""Refresh embedded manuals as a complete snapshot. Requires Python 3.10+ and Git."""

import argparse
import json
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

from resource_snapshot import publish_snapshot

ROOT = Path(__file__).resolve().parents[1]
SOURCE_URL = "https://github.com/VictoriaMetrics/vmdocs.git"


def copy_docs(source: Path, staged: Path) -> None:
    """Copy selected manuals; the model-selection guide is intentionally link-only."""
    shutil.copytree(source / "content/anomaly-detection", staged / "anomaly-detection")
    excluded = staged / "anomaly-detection/guides/model-selection"
    if excluded.is_dir():
        shutil.rmtree(excluded)
    for name, candidates in {
        "metricsql.md": ("victoriametrics/MetricsQL.md", "victoriametrics/metricsql.md"),
        "logsql.md": ("victorialogs/logsql.md", "victorialogs/LogSQL.md"),
        "logsql-examples.md": ("victorialogs/logsql-examples.md", "victorialogs/LogSQL-examples.md"),
    }.items():
        match = next((source / "content" / p for p in candidates if (source / "content" / p).is_file()), None)
        if match is None:
            raise FileNotFoundError(f"Missing source manual: {name}")
        shutil.copyfile(match, staged / name)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--revision", default="main", help="Source branch or tag; resolved commit is recorded")
    args = parser.parse_args()
    scratch = ROOT / "tmp/resource-sync"
    scratch.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, TMPDIR=str(scratch), TMP=str(scratch), TEMP=str(scratch))

    def populate(staged: Path) -> None:
        with tempfile.TemporaryDirectory(prefix="source-", dir=scratch) as work:
            source = Path(work) / "source"
            subprocess.run(["git", "clone", "--depth=1", "--branch", args.revision,
                            SOURCE_URL, str(source)], check=True, env=env)
            revision = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
            copy_docs(source, staged)
            (staged / "snapshot.json").write_text(json.dumps({"repository": SOURCE_URL, "revision": revision}, indent=2) + "\n")

    publish_snapshot(ROOT / "internal/resources/docs", populate)
    print("Documentation snapshot updated.")


if __name__ == "__main__":
    main()
