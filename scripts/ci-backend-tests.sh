#!/usr/bin/env bash
set -euo pipefail

shard="${1:?expected core or service shard index}"
shard_count="${2:?expected service shard count}"
service_package="$(go list ./internal/services)"
test_flags=(-v -race -timeout=30m -coverprofile=coverage.out -covermode=atomic)

if [[ "$shard" == core ]]; then
  package_list="$(go list ./...)"
  packages=()
  while IFS= read -r package; do
    if [[ "$package" != "$service_package" ]]; then
      packages+=("$package")
    fi
  done <<< "$package_list"
  ((${#packages[@]} > 0))
  go test "${packages[@]}" "${test_flags[@]}"
  exit
fi

[[ "$shard" =~ ^[0-9]+$ && "$shard_count" =~ ^[1-9][0-9]*$ ]]
((shard < shard_count))
# Discover runnable names from Go itself so new tests automatically join a group.
# Each top-level test and all its subtests run in exactly one isolated process.
test_list="$(go test "$service_package" -list .)"
names=()
while IFS= read -r name; do
  if [[ "$name" =~ ^(Test|Example|Fuzz)[[:alnum:]_]*$ ]]; then
    names+=("$name")
  fi
done <<< "$test_list"
((${#names[@]} >= shard_count))
selected=()
for ((index = shard; index < ${#names[@]}; index += shard_count)); do
  selected+=("${names[index]}")
done
pattern="$(IFS='|'; echo "${selected[*]}")"
echo "Service group $shard/$shard_count: ${#selected[@]} of ${#names[@]} tests"
go test "$service_package" "${test_flags[@]}" -run "^($pattern)$"
