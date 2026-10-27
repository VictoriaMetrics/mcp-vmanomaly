"""Stage complete resource directories before replacement; restore the old snapshot on failure."""

import fcntl
import os
import shutil
import tempfile
from collections.abc import Callable
from pathlib import Path


def publish_snapshot(destination: Path, populate: Callable[[Path], None]) -> None:
    """Serialize refreshes and replace whole filesets, never modifying the live tree in place.

    The two directory renames have a short absence window: run refreshes before
    builds, not concurrently with them. On publication failure the old directory
    is restored. An interrupted process leaves its backup in repository-local tmp.
    """
    destination = destination.resolve()
    repository = Path(__file__).resolve().parents[1]
    scratch = repository / 'tmp' / 'resource-sync'
    scratch.mkdir(parents=True, exist_ok=True)
    # Only two destinations are owned by these scripts: docs and guides.
    if destination not in {repository / 'internal/resources/docs', repository / 'internal/resources/guides'}:
        raise ValueError('Resource destination must be the repository docs or guides directory')
    with (scratch / f'{destination.name}.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        workdir = Path(tempfile.mkdtemp(prefix=f'{destination.name}-', dir=scratch))
        staged, backup = workdir / 'staged', workdir / 'previous'
        try:
            staged.mkdir()
            populate(staged)  # Finish fetching and validation before touching the installed snapshot.
            try:
                if destination.exists():
                    os.replace(destination, backup)
                os.replace(staged, destination)
            except BaseException:
                if backup.exists():
                    os.replace(backup, destination)
                raise
            shutil.rmtree(backup, ignore_errors=True)
        finally:
            # A failed rollback must never delete the only surviving old snapshot.
            if not backup.exists():
                shutil.rmtree(workdir)
