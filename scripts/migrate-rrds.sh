#!/bin/bash
#
# migrate-rrds.sh - Migrate flat RRD file layout to per-host subdirectories
#
# Before: {rrdDir}/{hostname}_latency.rrd
# After:  {rrdDir}/{hostname}/ping.rrd
#
# The RRD data source name inside the file remains "latency" (unchanged).
# Only the file's location and name change.
#
# Usage: ./migrate-rrds.sh /path/to/rrds [--dry-run]
#
# The script is idempotent: files already in the new layout are skipped.
# Original files are preserved until you confirm the migration looks correct.

set -euo pipefail

RRD_DIR="${1:?Usage: $0 <rrd-directory> [--dry-run]}"
DRY_RUN=false

if [[ "${2:-}" == "--dry-run" ]]; then
    DRY_RUN=true
    echo "=== DRY RUN MODE ==="
fi

if [[ ! -d "$RRD_DIR" ]]; then
    echo "Error: $RRD_DIR is not a directory"
    exit 1
fi

migrated=0
skipped=0
errors=0

# Mapping of old metric suffixes to new check type filenames
# Add entries here if you add more check types in the future
declare -A METRIC_TO_CHECK=(
    ["latency"]="ping"
)

# Find all .rrd files directly in the rrdDir (not in subdirectories)
for rrd_file in "$RRD_DIR"/*_*.rrd; do
    # Handle case where glob matches nothing
    [[ -f "$rrd_file" ]] || continue

    filename=$(basename "$rrd_file")
    # Split on the LAST underscore to handle hostnames with underscores
    # e.g., "my_host_latency.rrd" -> hostname="my_host", old_metric="latency"
    metric_with_ext="${filename##*_}"
    old_metric="${metric_with_ext%.rrd}"
    hostname="${filename%_${metric_with_ext}}"

    if [[ -z "$hostname" || -z "$old_metric" ]]; then
        echo "SKIP (couldn't parse): $rrd_file"
        skipped=$((skipped + 1))
        continue
    fi

    # Look up the new check type name for this metric
    new_check="${METRIC_TO_CHECK[$old_metric]:-}"
    if [[ -z "$new_check" ]]; then
        echo "SKIP (unknown metric '$old_metric'): $rrd_file"
        skipped=$((skipped + 1))
        continue
    fi

    dest_dir="$RRD_DIR/$hostname"
    dest_file="$dest_dir/$new_check.rrd"

    if [[ -f "$dest_file" ]]; then
        echo "SKIP (already exists): $dest_file"
        skipped=$((skipped + 1))
        continue
    fi

    if $DRY_RUN; then
        echo "WOULD MOVE: $rrd_file -> $dest_file"
    else
        mkdir -p "$dest_dir"
        cp -p "$rrd_file" "$dest_file"
        if [[ $? -eq 0 ]]; then
            echo "MIGRATED: $rrd_file -> $dest_file"
            migrated=$((migrated + 1))
        else
            echo "ERROR: Failed to copy $rrd_file"
            errors=$((errors + 1))
        fi
    fi
done

echo ""
echo "=== Summary ==="
echo "Migrated: $migrated"
echo "Skipped:  $skipped"
echo "Errors:   $errors"

if [[ $migrated -gt 0 ]] && ! $DRY_RUN; then
    echo ""
    echo "Migration complete. Original files are still in place."
    echo "After verifying the new layout works correctly, remove the old flat files with:"
    echo ""
    echo "  rm $RRD_DIR/*_*.rrd"
    echo ""
    echo "Or to do a dry-run of that cleanup:"
    echo ""
    echo "  ls $RRD_DIR/*_*.rrd"
fi
