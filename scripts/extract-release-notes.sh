#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <release-tag>" >&2
  exit 2
fi

tag="$1"
changelog="${CHANGELOG_FILE:-CHANGELOG.md}"

if [[ ! -f "${changelog}" ]]; then
  echo "error: changelog not found: ${changelog}" >&2
  exit 1
fi

awk -v heading="## [${tag}]" '
  index($0, heading) == 1 {
    found = 1
    next
  }
  found && /^## \[/ {
    exit
  }
  found && /^\[[^]]+\]:/ {
    exit
  }
  found && !emitted && $0 == "" {
    next
  }
  found {
    print
    emitted = 1
  }
  END {
    if (!found) {
      print "error: no changelog section found for " heading > "/dev/stderr"
      exit 1
    }
  }
' "${changelog}"
