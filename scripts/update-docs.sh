#!/usr/bin/env bash
set -euo pipefail

# Update vmanomaly documentation from VictoriaMetrics docs repository

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
DOCS_DIR="${REPO_ROOT}/internal/resources/docs"
TMP_DIR="$(mktemp -d /tmp/vmdocs-temp.XXXXXX)"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

copy_doc_file() {
  local destination_name="$1"
  shift

  local source_rel_path
  for source_rel_path in "$@"; do
    if [ -f "${TMP_DIR}/${source_rel_path}" ]; then
      cp "${TMP_DIR}/${source_rel_path}" "${DOCS_DIR}/${destination_name}"
      return 0
    fi
  done

  echo "error: none of candidate source files exist for ${destination_name}" >&2
  printf 'checked:\n' >&2
  printf '  %s\n' "$@" >&2
  return 1
}

# Remove existing docs to avoid stale files after copy
rm -rf "${DOCS_DIR}/anomaly-detection" "${DOCS_DIR}/metricsql" "${DOCS_DIR}/logsql"
rm -f \
  "${DOCS_DIR}/metricsql.md" \
  "${DOCS_DIR}/MetricsQL.md" \
  "${DOCS_DIR}/logsql.md" \
  "${DOCS_DIR}/logsql-examples.md"

# Clone VictoriaMetrics docs repository with sparse checkout
git clone --no-checkout --depth=1 https://github.com/VictoriaMetrics/vmdocs.git "${TMP_DIR}"

# Setup sparse checkout with only directories that contain required files
git -C "${TMP_DIR}" sparse-checkout init --cone
git -C "${TMP_DIR}" sparse-checkout set \
  content/anomaly-detection \
  content/victoriametrics \
  content/victorialogs
git -C "${TMP_DIR}" checkout main

# Copy /anomaly-detection docs to our resources
mkdir -p "${DOCS_DIR}"
cp -r "${TMP_DIR}/content/anomaly-detection" "${DOCS_DIR}/"
# Copy fresh manuals for MetricsQL and LogsQL as markdown files
copy_doc_file "metricsql.md" \
  "content/victoriametrics/MetricsQL.md" \
  "content/victoriametrics/metricsql.md"
copy_doc_file "logsql.md" \
  "content/victorialogs/logsql.md" \
  "content/victorialogs/LogSQL.md"
copy_doc_file "logsql-examples.md" \
  "content/victorialogs/logsql-examples.md" \
  "content/victorialogs/LogSQL-examples.md"

echo "✅ Documentation updated successfully!"
echo "📁 Location: internal/resources/docs/anomaly-detection"
