#!/usr/bin/env bash

set -euo pipefail

fail() {
  echo "Error: $*" >&2
  exit 1
}

[[ $# -eq 2 ]] ||
  fail "Usage: $0 <source-docs-directory> <destination-docs-directory>"

source_docs=$1
destination_docs=$2

[[ -d $source_docs && ! -L $source_docs ]] ||
  fail "Source documentation directory is missing, invalid, or symlinked: $source_docs"

canonical_source="$(readlink -f -- "$source_docs")" ||
  fail "Unable to resolve source documentation directory: $source_docs"
canonical_destination="$(readlink -m -- "$destination_docs")" ||
  fail "Unable to resolve destination documentation path: $destination_docs"
source_prefix="${canonical_source%/}/"
if [[ $canonical_destination == "$canonical_source" ||
  $canonical_destination == "$source_prefix"* ]]; then
  fail "Destination documentation path must not equal or be beneath the source: $destination_docs"
fi

symlink_path="$(find -P "$source_docs" -type l -print -quit)" ||
  fail "Unable to inspect source documentation directory: $source_docs"
if [[ -n $symlink_path ]]; then
  fail "Source documentation must not contain symlinks: $source_docs"
fi

if [[ -e $destination_docs || -L $destination_docs ]]; then
  fail "Destination documentation path must not already exist: $destination_docs"
fi

destination_created=false
operation_complete=false
cleanup() {
  local original_status=$?

  trap - EXIT
  if [[ $original_status -ne 0 && $destination_created == true &&
    $operation_complete == false ]]; then
    if ! rm -rf -- "$destination_docs"; then
      echo "Warning: unable to remove incomplete documentation destination: $destination_docs" >&2
    fi
  fi
  exit "$original_status"
}
trap cleanup EXIT

mkdir -m 0755 -- "$destination_docs" ||
  fail "Unable to create fresh destination documentation directory: $destination_docs"
destination_created=true

shopt -s dotglob nullglob
for entry in "$source_docs"/*; do
  if [[ ${entry##*/} == plans && -d $entry ]]; then
    continue
  fi
  cp -a -- "$entry" "$destination_docs/" ||
    fail "Unable to stage documentation entry: $entry"
done

operation_complete=true
trap - EXIT
