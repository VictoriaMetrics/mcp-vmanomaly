"""Exercise sync output/provenance and stale checks without network access."""
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import resource_snapshot

spec = importlib.util.spec_from_file_location("sync_guide", Path(__file__).resolve().parents[1] / "sync-preset-guide.py")
sync = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sync)


class GuideSyncTests(unittest.TestCase):
    def setUp(self):
        scratch = Path(__file__).resolve().parents[2] / "tmp/resource-tests"
        scratch.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / "skills"
        self.source.mkdir()
        self.git("init", "-q")
        self.git("config", "user.name", "Fixture")
        self.git("config", "user.email", "fixture@example.invalid")
        self.git("remote", "add", "origin", "https://github.com/VictoriaMetrics/skills.git")
        guide = self.source / sync.SOURCE
        guide.parent.mkdir(parents=True)
        guide.write_text("# Canonical fixture\n")
        self.git("add", ".")
        self.git("commit", "-qm", "fixture")
        self.git("update-ref", "refs/remotes/origin/main", "HEAD")
        self.destination = self.root / "internal/resources/guides"
        self.destination.mkdir(parents=True)
        for mocked in (patch.object(sync, "DESTINATION", self.destination),
                       patch.object(resource_snapshot, "__file__", str(self.root / "scripts/resource_snapshot.py"))):
            mocked.start()
            self.addCleanup(mocked.stop)

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.source), *args], text=True).strip()

    def run_sync(self, *args):
        with patch.object(sys, "argv", ["sync", str(self.source), *args]):
            sync.main()

    def test_generation_check_and_stale_file(self):
        (self.destination / "obsolete.md").write_text("old")
        self.run_sync()
        self.assertFalse((self.destination / "obsolete.md").exists())
        self.assertEqual((self.destination / "preset-v1.md").read_bytes(), (self.source / sync.SOURCE).read_bytes())
        metadata = json.loads((self.destination / "preset-v1.json").read_text())
        self.assertEqual(metadata["source_revision"], self.git("rev-parse", "HEAD"))
        self.assertEqual(metadata["source_status"], "merged")
        self.run_sync("--check")
        (self.destination / "preset-v1.md").write_text("stale")
        with self.assertRaisesRegex(SystemExit, "stale"):
            self.run_sync("--check")
        self.run_sync()
        (self.destination / "extra.md").write_text("extra")
        with self.assertRaisesRegex(SystemExit, "unexpected"):
            self.run_sync("--check")

    def test_rejects_dirty_source_and_wrong_origin(self):
        (self.source / sync.SOURCE).write_text("uncommitted")
        with self.assertRaisesRegex(SystemExit, "Commit canonical"):
            self.run_sync()
        self.git("remote", "set-url", "origin", "https://github.com/example/fork.git")
        with self.assertRaisesRegex(SystemExit, "canonical checkout"):
            self.run_sync()

    def test_unmerged_source_requires_explicit_review_override(self):
        self.git("update-ref", "-d", "refs/remotes/origin/main")
        with self.assertRaisesRegex(SystemExit, "merged revision"):
            self.run_sync()
        self.run_sync("--allow-unmerged")
        metadata = json.loads((self.destination / "preset-v1.json").read_text())
        self.assertEqual(metadata["source_status"], "unmerged-review")


if __name__ == "__main__":
    unittest.main()
