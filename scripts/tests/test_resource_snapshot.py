"""Offline regression coverage for whole-snapshot publication and manual selection."""
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import resource_snapshot
from update_docs import copy_docs


class SnapshotTests(unittest.TestCase):
    def setUp(self):
        scratch = Path(__file__).resolve().parents[2] / "tmp/resource-tests"
        scratch.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.destination = self.root / "internal/resources/docs"
        self.destination.mkdir(parents=True)
        (self.destination / "old.md").write_text("old")
        fake_script = self.root / "scripts/resource_snapshot.py"
        self.mock_root = patch.object(resource_snapshot, "__file__", str(fake_script))
        self.mock_root.start()
        self.addCleanup(self.mock_root.stop)

    def publish(self, populate):
        resource_snapshot.publish_snapshot(self.destination, populate)

    def populate(self, staged):
        (staged / "new.md").write_text("new")

    def test_complete_replacement_removes_stale_files(self):
        self.publish(self.populate)
        self.assertEqual([p.name for p in self.destination.iterdir()], ["new.md"])

    def test_population_failure_preserves_previous(self):
        def fail(staged):
            self.populate(staged)
            raise ValueError("incomplete download")
        with self.assertRaises(ValueError):
            self.publish(fail)
        self.assertEqual((self.destination / "old.md").read_text(), "old")

    def test_publication_failure_rolls_back(self):
        real_replace = os.replace
        def replace(source, destination):
            if Path(source).name == "staged":
                raise OSError("publication failed")
            real_replace(source, destination)
        with patch.object(resource_snapshot.os, "replace", side_effect=replace):
            with self.assertRaises(OSError):
                self.publish(self.populate)
        self.assertEqual((self.destination / "old.md").read_text(), "old")

    def test_failed_rollback_keeps_recovery_copy(self):
        real_replace = os.replace
        def replace(source, destination):
            if Path(source).name in {"staged", "previous"}:
                raise OSError("filesystem unavailable")
            real_replace(source, destination)
        with patch.object(resource_snapshot.os, "replace", side_effect=replace):
            with self.assertRaises(OSError):
                self.publish(self.populate)
        backups = list((self.root / "tmp/resource-sync").glob("docs-*/previous/old.md"))
        self.assertEqual(len(backups), 1)
        self.assertEqual(backups[0].read_text(), "old")

    def test_concurrent_refresh_fails_without_waiting(self):
        def nested(staged):
            with self.assertRaises(BlockingIOError):
                self.publish(self.populate)
            self.populate(staged)
        self.publish(nested)

    def test_missing_manual_does_not_replace_snapshot(self):
        source = self.root / "source"
        (source / "content/anomaly-detection").mkdir(parents=True)
        with self.assertRaises(FileNotFoundError):
            self.publish(lambda staged: copy_docs(source, staged))
        self.assertTrue((self.destination / "old.md").exists())

    def test_selected_docs_exclude_external_model_guide(self):
        source = self.root / "source"
        for relative in ["anomaly-detection/README.md", "anomaly-detection/guides/model-selection/README.md",
                         "victoriametrics/MetricsQL.md", "victorialogs/logsql.md", "victorialogs/logsql-examples.md"]:
            target = source / "content" / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text("manual")
        self.publish(lambda staged: copy_docs(source, staged))
        self.assertTrue((self.destination / "metricsql.md").exists())
        self.assertFalse((self.destination / "anomaly-detection/guides/model-selection").exists())


if __name__ == "__main__":
    unittest.main()
