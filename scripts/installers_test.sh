#!/usr/bin/env bash

set -euo pipefail

repository_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
system_installer="$repository_root/install.sh"
user_installer="$repository_root/install-user-space.sh"
system_unit="$repository_root/LumenForge.service"
docs_staging_helper="$repository_root/scripts/stage-release-docs.sh"

fail() {
  echo "installer test failed: $*" >&2
  exit 1
}

assert_contains() {
  local file=$1
  local text=$2
  grep -Fq -- "$text" "$file" ||
    fail "$file does not contain required text: $text"
}

assert_absent() {
  local file=$1
  local text=$2
  if grep -Fq -- "$text" "$file"; then
    fail "$file contains forbidden text: $text"
  fi
}

assert_absent_pattern() {
  local file=$1
  local pattern=$2
  if grep -Eq -- "$pattern" "$file"; then
    fail "$file contains forbidden pattern: $pattern"
  fi
}

line_number() {
  local file=$1
  local text=$2
  local line remainder

  while IFS=: read -r line remainder; do
    printf '%s' "$line"
    return 0
  done < <(grep -nF -- "$text" "$file")
  fail "$file does not contain ordered text: $text"
}

assert_order() {
  local file=$1
  local before=$2
  local after=$3
  local before_line after_line

  before_line="$(line_number "$file" "$before")"
  after_line="$(line_number "$file" "$after")"
  ((before_line < after_line)) ||
    fail "$file places '$after' before required predecessor '$before'"
}

for installer in "$system_installer" "$user_installer"; do
  assert_contains "$installer" '[[ ! -L $INSTALL_DIR ]]'
  assert_contains "$installer" 'mktemp -d /opt/.LumenForge.stage.XXXXXX'
  assert_contains "$installer" 'for directory in web static api openrgb; do'
  assert_absent "$installer" 'for directory in web static docs api openrgb; do'
  assert_contains "$installer" 'scripts/stage-release-docs.sh'
  assert_contains "$installer" 'bash "$SOURCE_DIR/scripts/stage-release-docs.sh" "$SOURCE_DIR/docs" "$STAGING_DIR/docs"'
  assert_absent "$installer" 'install -d -o root -g root -m 0755 "$STAGING_DIR/docs"'
  assert_absent "$installer" 'find "$SOURCE_DIR/docs" -mindepth 1 -maxdepth 1'
  assert_contains "$installer" 'chown -R root:root "$STAGING_DIR"'
  assert_contains "$installer" 'find "$STAGING_DIR" -type d -exec chmod 0755'
  assert_contains "$installer" 'find "$STAGING_DIR" -type f -exec chmod 0644'
  assert_contains "$installer" 'database/lcd/background.jpg'
  assert_contains "$installer" 'database/rgb.json'
  assert_absent "$installer" 'WorkingDirectory='
  assert_absent "$installer" 'chown -R "$TARGET_USER'
  assert_absent "$installer" 'chown -R "$RUNTIME_USER'
  assert_absent "$installer" '$STAGING_DIR/database/profiles'
  assert_absent "$installer" '$STAGING_DIR/config.json'
  assert_absent "$installer" '$SOURCE_DIR/install.sh'
done

assert_contains "$system_installer" '[[ $EUID -ne 0 ]]'
assert_contains "$system_installer" 'for command in bash readlink getent install cp find mkdir'
assert_absent "$system_installer" 'usermod '
assert_contains "$system_installer" 'validate_runtime_group_entry'
assert_contains "$system_installer" '$entry =~ ^[^:]+:[^:]*:[^:]+:[^:]*$'
assert_contains "$system_installer" 'group_gid -ne 0'
assert_contains "$system_installer" 'group_gid -gt 0'
assert_contains "$system_installer" 'group uses privileged GID 0'
assert_contains "$system_installer" 'group data is incomplete or malformed'
assert_absent "$system_installer" '-z $group_members'
assert_contains "$system_installer" 'passwd_entry="$(getent passwd "$RUNTIME_USER")"'
assert_contains "$system_installer" 'account_uid -ne 0'
assert_contains "$system_installer" 'account_gid -eq "$RUNTIME_GROUP_GID"'
assert_contains "$system_installer" 'account_home == "$STATE_DIR"'
assert_contains "$system_installer" 'account_shell == "$NOLOGIN_SHELL"'
assert_contains "$system_installer" 'Existing $RUNTIME_USER account is not the dedicated LumenForge service identity'
assert_contains "$system_installer" "-M -p '!'"
assert_contains "$system_installer" 'STATE_DIR="/var/lib/lumenforge"'
assert_contains "$system_installer" 'install -d -o "$RUNTIME_USER" -g "$RUNTIME_GROUP" -m 0750 "$STATE_DIR"'
assert_contains "$system_installer" '[[ ! -L $STATE_DIR && -d $STATE_DIR ]]'
assert_contains "$system_installer" 'Refusing symlinked or non-directory state root'
assert_contains "$system_installer" '$(stat -c '\''%U:%G:%a'\'' "$STATE_DIR") == "$RUNTIME_USER:$RUNTIME_GROUP:750"'
assert_contains "$system_installer" 'refusing to normalize service-owned state'
assert_absent_pattern "$system_installer" 'chown[[:space:]]+-R.*STATE_DIR'
assert_absent_pattern "$system_installer" 'chmod[[:space:]]+-R.*STATE_DIR'
assert_absent "$system_installer" '"$STATE_DIR/database/'
assert_contains "$system_installer" 'Environment=LUMENFORGE_SERVICE_MODE=system'
assert_contains "$system_installer" 'Environment=LUMENFORGE_CONFIG_ROOT=/var/lib/lumenforge'
assert_contains "$system_installer" 'Environment=LUMENFORGE_DATA_ROOT=/var/lib/lumenforge'
assert_contains "$system_installer" 'UMask=0077'
assert_contains "$system_installer" 'check_user_service_conflict'
assert_contains "$system_installer" 'systemctl stop "$PRODUCT.service"'
assert_contains "$system_installer" 'Refusing to replace non-regular or symlinked system unit destination'
assert_contains "$system_installer" 'UNIT_TEMP="$(mktemp /etc/systemd/system/.LumenForge.service.write.XXXXXX)"'
assert_contains "$system_installer" 'exec {unit_fd}>"$UNIT_TEMP"'
assert_contains "$system_installer" 'if ! exec {unit_fd}>&-; then'
assert_contains "$system_installer" 'install -o root -g root -m 0644 "$UNIT_TEMP" "$UNIT_READY"'
assert_contains "$system_installer" 'mv -fT -- "$UNIT_READY" "$SYSTEMD_FILE"'
assert_absent "$system_installer" '>"$SYSTEMD_FILE"'
assert_contains "$system_installer" 'mv -- "$PREVIOUS_DIR" "$INSTALL_DIR"'
assert_contains "$system_installer" 'mv -fT -- "$UNIT_BACKUP" "$SYSTEMD_FILE"'
assert_contains "$system_installer" 'systemctl daemon-reload'
assert_contains "$system_installer" 'unable to restore the previously active $PRODUCT.service'
assert_order "$system_installer" 'echo "Starting $PRODUCT system service..."' 'if rm -rf -- "$PREVIOUS_DIR"; then'
assert_order "$system_installer" 'systemctl is-active --quiet "$PRODUCT.service"' 'if [[ -e $STATE_DIR || -L $STATE_DIR ]]; then'
assert_order "$system_installer" 'systemctl stop "$PRODUCT.service"' 'if [[ -e $STATE_DIR || -L $STATE_DIR ]]; then'

assert_contains "$user_installer" '[[ $EUID -eq 0 ]]'
unprivileged_validation="$(
  sed -n '/^for command in readlink /,/^done$/p' "$user_installer"
)"
[[ $unprivileged_validation != *groupadd* ]] ||
  fail "$user_installer requires groupadd through the desktop user's PATH"
[[ $unprivileged_validation != *usermod* ]] ||
  fail "$user_installer requires usermod through the desktop user's PATH"
assert_contains "$user_installer" '"${PRIVILEGED_CMD[@]}" bash -c'
assert_contains "$user_installer" 'for command in bash getent id groupadd usermod udevadm chown install cp find'
assert_contains "$user_installer" 'mkdir mktemp mv readlink rm rmdir stat chmod; do'
assert_contains "$user_installer" 'privileged command $command is required'
assert_order "$user_installer" '"${PRIVILEGED_CMD[@]}" bash -c' 'systemctl --user stop "$PRODUCT.service"'
assert_order "$user_installer" '"${PRIVILEGED_CMD[@]}" bash -c' 'groupadd -r "$DEVICE_GROUP"'
assert_contains "$user_installer" 'validate_device_group_entry'
assert_contains "$user_installer" '$entry =~ ^[^:]+:[^:]*:[^:]+:[^:]*$'
assert_contains "$user_installer" 'group_gid -ne 0'
assert_contains "$user_installer" 'group_gid -gt 0'
assert_contains "$user_installer" 'group uses privileged GID 0'
assert_contains "$user_installer" 'group data is incomplete or malformed'
assert_absent "$user_installer" '-z $group_members'
assert_contains "$user_installer" 'user_was_in_device_group=false'
assert_contains "$user_installer" 'user_was_in_device_group=true'
assert_contains "$user_installer" 'if [[ $user_was_in_device_group == false ]]; then'
assert_order "$user_installer" 'user_was_in_device_group=false' 'if [[ $user_was_in_device_group == false ]]; then'
assert_absent "$user_installer" 'user_in_device_group'
assert_contains "$user_installer" 'target_groups="$(id -nG "$TARGET_USER")"'
assert_contains "$user_installer" 'if [[ " $target_groups " != *" $DEVICE_GROUP "* ]]; then'
assert_order "$user_installer" 'validate_device_group_entry "$group_entry"' 'usermod -aG "$DEVICE_GROUP" "$TARGET_USER"'
assert_order "$user_installer" 'target_groups="$(id -nG "$TARGET_USER")"' 'usermod -aG "$DEVICE_GROUP" "$TARGET_USER"'
assert_contains "$user_installer" 'if [[ $session_has_device_group == true ]]; then'
assert_order "$user_installer" 'if [[ $session_has_device_group == true ]]; then' 'echo "Starting $PRODUCT user service..."'
assert_contains "$user_installer" 'CONFIG_HOME="${XDG_CONFIG_HOME:-$USER_HOME/.config}"'
assert_contains "$user_installer" 'DATA_HOME="${XDG_DATA_HOME:-$USER_HOME/.local/share}"'
assert_contains "$user_installer" '[[ $CONFIG_HOME == /* ]]'
assert_contains "$user_installer" '[[ $DATA_HOME == /* ]]'
assert_contains "$user_installer" 'install -d -m 0700 "$CONFIG_ROOT" "$DATA_ROOT"'
assert_contains "$user_installer" 'Environment=LUMENFORGE_SERVICE_MODE=user'
assert_contains "$user_installer" 'Environment="LUMENFORGE_CONFIG_ROOT=$escaped_config_root"'
assert_contains "$user_installer" 'Environment="LUMENFORGE_DATA_ROOT=$escaped_data_root"'
assert_contains "$user_installer" 'UMask=0077'
assert_contains "$user_installer" 'systemctl is-active --quiet "$PRODUCT.service"'
assert_contains "$user_installer" 'systemctl --user stop "$PRODUCT.service"'
assert_contains "$user_installer" 'GROUP="lumenforge"'
assert_contains "$user_installer" 'Refusing to replace non-regular or symlinked user unit destination'
assert_contains "$user_installer" 'UNIT_TEMP="$(mktemp "$SYSTEMD_DIR/.LumenForge.service.write.XXXXXX")"'
assert_contains "$user_installer" 'exec {unit_fd}>"$UNIT_TEMP"'
assert_contains "$user_installer" 'if ! exec {unit_fd}>&-; then'
assert_contains "$user_installer" 'install -m 0600 "$UNIT_TEMP" "$UNIT_READY"'
assert_contains "$user_installer" '$(stat -c '\''%u:%a'\'' "$UNIT_READY") == "$TARGET_UID:600"'
assert_contains "$user_installer" 'mv -fT -- "$UNIT_READY" "$SYSTEMD_FILE"'
assert_absent "$user_installer" '>"$SYSTEMD_FILE"'
assert_contains "$user_installer" 'rollback_application_tree'
assert_contains "$user_installer" 'mv -- "$PREVIOUS_DIR" "$INSTALL_DIR"'
assert_contains "$user_installer" 'mv -fT -- "$UNIT_BACKUP" "$SYSTEMD_FILE"'
assert_contains "$user_installer" 'systemctl --user daemon-reload'
assert_contains "$user_installer" 'unable to restore the previously active user service'
assert_order "$user_installer" 'systemctl --user enable "$PRODUCT.service"' '"${PRIVILEGED_CMD[@]}" rm -rf -- "$PREVIOUS_DIR" "$SWAP_MARKER"'
assert_order "$user_installer" 'echo "Starting $PRODUCT user service..."' '"${PRIVILEGED_CMD[@]}" rm -rf -- "$PREVIOUS_DIR" "$SWAP_MARKER"'

assert_contains "$system_unit" 'Environment=LUMENFORGE_SERVICE_MODE=system'
assert_contains "$system_unit" 'Environment=LUMENFORGE_APPLICATION_ROOT=/opt/LumenForge'
assert_contains "$system_unit" 'Environment=LUMENFORGE_CONFIG_ROOT=/var/lib/lumenforge'
assert_contains "$system_unit" 'Environment=LUMENFORGE_DATA_ROOT=/var/lib/lumenforge'
assert_contains "$system_unit" 'UMask=0077'
assert_absent "$system_unit" 'WorkingDirectory='

documentation_test_root="$(mktemp -d "${TMPDIR:-/tmp}/lumenforge-docs-test.XXXXXX")"
trap 'rm -rf -- "$documentation_test_root"' EXIT
source_docs="$documentation_test_root/source/docs"
destination_docs="$documentation_test_root/destination/docs"
install -d -m 0755 "$source_docs/plans" "$source_docs/guides/nested" \
  "$documentation_test_root/destination"

configuration_content="configuration fixture"
releasing_content="releasing fixture"
plan_content="maintenance plan fixture"
nested_content="nested documentation fixture"
printf '%s\n' "$configuration_content" >"$source_docs/configuration.md"
printf '%s\n' "$releasing_content" >"$source_docs/releasing.md"
printf '%s\n' "$plan_content" \
  >"$source_docs/plans/maintenance-and-reliability-backlog.md"
printf '%s\n' "$nested_content" >"$source_docs/guides/nested/example.md"

bash "$docs_staging_helper" "$source_docs" "$destination_docs"

[[ $(stat -c '%a' "$destination_docs") == 755 ]] ||
  fail "fresh documentation destination does not have mode 0755"
[[ $(<"$destination_docs/configuration.md") == "$configuration_content" ]] ||
  fail "ordinary user-facing documentation was not staged"
[[ $(<"$destination_docs/releasing.md") == "$releasing_content" ]] ||
  fail "second ordinary user-facing document was not staged"
[[ $(<"$destination_docs/guides/nested/example.md") == "$nested_content" ]] ||
  fail "nested ordinary documentation was not staged"
[[ ! -e $destination_docs/plans && ! -L $destination_docs/plans ]] ||
  fail "maintainer planning documentation was staged"
[[ $(<"$source_docs/configuration.md") == "$configuration_content" ]] ||
  fail "source configuration documentation was changed"
[[ $(<"$source_docs/releasing.md") == "$releasing_content" ]] ||
  fail "source release documentation was changed"
[[ $(<"$source_docs/plans/maintenance-and-reliability-backlog.md") == "$plan_content" ]] ||
  fail "source maintainer documentation was changed"
[[ $(<"$source_docs/guides/nested/example.md") == "$nested_content" ]] ||
  fail "source nested documentation was changed"

real_cp="$(type -P cp)"
[[ -n $real_cp ]] || fail "unable to resolve the real cp command for testing"
cp_shim_directory="$documentation_test_root/cp-shim"
cp_shim_state="$cp_shim_directory/call-count"
forced_failure_destination="$documentation_test_root/forced-failure-destination"
install -d -m 0755 "$cp_shim_directory"
cat >"$cp_shim_directory/cp" <<'CP_SHIM'
#!/usr/bin/env bash
set -euo pipefail

call_count=0
if [[ -f $CP_SHIM_STATE ]]; then
  call_count="$(<"$CP_SHIM_STATE")"
fi
call_count=$((call_count + 1))
printf '%s\n' "$call_count" >"$CP_SHIM_STATE"

if [[ $call_count -eq 1 ]]; then
  exec "$REAL_CP" "$@"
fi
echo "forced cp failure after first successful copy" >&2
exit 23
CP_SHIM
chmod 0755 "$cp_shim_directory/cp"

if PATH="$cp_shim_directory:$PATH" REAL_CP="$real_cp" \
  CP_SHIM_STATE="$cp_shim_state" \
  bash "$docs_staging_helper" "$source_docs" "$forced_failure_destination" \
  >/dev/null 2>&1; then
  fail "documentation staging succeeded despite a forced mid-copy failure"
fi
[[ $(<"$cp_shim_state") == 2 ]] ||
  fail "forced copy failure did not occur after one successful copy"
[[ ! -e $forced_failure_destination && ! -L $forced_failure_destination ]] ||
  fail "failed documentation staging left a partial destination"
[[ $(<"$source_docs/configuration.md") == "$configuration_content" ]] ||
  fail "forced copy failure changed source configuration documentation"
[[ $(<"$source_docs/releasing.md") == "$releasing_content" ]] ||
  fail "forced copy failure changed source release documentation"
[[ $(<"$source_docs/plans/maintenance-and-reliability-backlog.md") == "$plan_content" ]] ||
  fail "forced copy failure changed source maintainer documentation"
[[ $(<"$source_docs/guides/nested/example.md") == "$nested_content" ]] ||
  fail "forced copy failure changed source nested documentation"

if bash "$docs_staging_helper" "$source_docs" "$source_docs" >/dev/null 2>&1; then
  fail "documentation staging accepted the source as its destination"
fi
[[ $(<"$source_docs/configuration.md") == "$configuration_content" ]] ||
  fail "equal source and destination rejection changed source documentation"
[[ $(<"$source_docs/plans/maintenance-and-reliability-backlog.md") == "$plan_content" ]] ||
  fail "equal source and destination rejection changed source plans"

descendant_destination="$source_docs/generated/staged-docs"
if bash "$docs_staging_helper" "$source_docs" "$descendant_destination" \
  >/dev/null 2>&1; then
  fail "documentation staging accepted a destination beneath its source"
fi
[[ ! -e $descendant_destination && ! -L $descendant_destination ]] ||
  fail "rejected descendant destination was created"

symlinked_source_parent="$documentation_test_root/source-alias"
symlinked_parent_destination="$symlinked_source_parent/generated-docs"
ln -s -- "$source_docs" "$symlinked_source_parent"
if bash "$docs_staging_helper" "$source_docs" "$symlinked_parent_destination" \
  >/dev/null 2>&1; then
  fail "documentation staging accepted an overlapping destination through a symlinked parent"
fi
[[ ! -e $source_docs/generated-docs && ! -L $source_docs/generated-docs ]] ||
  fail "rejected symlink-parent destination was created beneath the source"

existing_empty_destination="$documentation_test_root/existing-empty-destination"
install -d -m 0755 "$existing_empty_destination"
if bash "$docs_staging_helper" "$source_docs" "$existing_empty_destination" \
  >/dev/null 2>&1; then
  fail "documentation staging accepted an existing empty destination"
fi
[[ -d $existing_empty_destination && ! -L $existing_empty_destination &&
  -z $(find "$existing_empty_destination" -mindepth 1 -print -quit) ]] ||
  fail "rejected empty destination was changed"

existing_file_destination="$documentation_test_root/existing-file-destination"
existing_file_content="existing destination file"
printf '%s\n' "$existing_file_content" >"$existing_file_destination"
if bash "$docs_staging_helper" "$source_docs" "$existing_file_destination" \
  >/dev/null 2>&1; then
  fail "documentation staging accepted an existing file destination"
fi
[[ $(<"$existing_file_destination") == "$existing_file_content" ]] ||
  fail "rejected file destination was changed"

existing_populated_destination="$documentation_test_root/existing-populated-destination"
install -d -m 0755 "$existing_populated_destination/plans"
existing_plan_content="existing destination plan"
printf '%s\n' "$existing_plan_content" \
  >"$existing_populated_destination/plans/existing-plan.md"
if bash "$docs_staging_helper" "$source_docs" "$existing_populated_destination" \
  >/dev/null 2>&1; then
  fail "documentation staging accepted an existing populated destination"
fi
[[ $(<"$existing_populated_destination/plans/existing-plan.md") == "$existing_plan_content" ]] ||
  fail "rejected populated destination was changed"
[[ ! -e $existing_populated_destination/configuration.md ]] ||
  fail "documentation was merged into a rejected existing destination"

if bash "$docs_staging_helper" "$source_docs" >/dev/null 2>&1; then
  fail "documentation staging accepted fewer than two arguments"
fi
if bash "$docs_staging_helper" "$source_docs" \
  "$documentation_test_root/excess-argument-destination" unexpected \
  >/dev/null 2>&1; then
  fail "documentation staging accepted more than two arguments"
fi

invalid_source="$documentation_test_root/not-a-directory"
printf 'not a directory\n' >"$invalid_source"
if bash "$docs_staging_helper" "$invalid_source" \
  "$documentation_test_root/invalid-destination" >/dev/null 2>&1; then
  fail "documentation staging accepted a non-directory source"
fi
if bash "$docs_staging_helper" "$documentation_test_root/missing-source" \
  "$documentation_test_root/missing-destination" >/dev/null 2>&1; then
  fail "documentation staging accepted a missing source"
fi

symlinked_source="$documentation_test_root/symlinked-source"
ln -s -- "$source_docs" "$symlinked_source"
if bash "$docs_staging_helper" "$symlinked_source" \
  "$documentation_test_root/symlinked-destination" >/dev/null 2>&1; then
  fail "documentation staging accepted a symlinked source directory"
fi

source_with_symlink="$documentation_test_root/source-with-symlink"
install -d -m 0755 "$source_with_symlink"
ln -s -- "$source_docs/configuration.md" "$source_with_symlink/configuration.md"
if bash "$docs_staging_helper" "$source_with_symlink" \
  "$documentation_test_root/nested-symlink-destination" >/dev/null 2>&1; then
  fail "documentation staging accepted a source containing a symlink"
fi

destination_target="$documentation_test_root/destination-target"
symlinked_destination="$documentation_test_root/symlinked-destination"
install -d -m 0755 "$destination_target"
ln -s -- "$destination_target" "$symlinked_destination"
if bash "$docs_staging_helper" "$source_docs" "$symlinked_destination" \
  >/dev/null 2>&1; then
  fail "documentation staging accepted a symlinked destination directory"
fi

dangling_destination="$documentation_test_root/dangling-destination"
ln -s -- "$documentation_test_root/missing-destination-target" "$dangling_destination"
if bash "$docs_staging_helper" "$source_docs" "$dangling_destination" \
  >/dev/null 2>&1; then
  fail "documentation staging accepted a dangling destination symlink"
fi
[[ -L $dangling_destination ]] ||
  fail "rejected dangling destination symlink was changed"

replace_unit_for_test() {
  local source=$1
  local destination=$2
  local mode=$3
  local directory temporary ready

  if [[ -e $destination || -L $destination ]]; then
    [[ ! -L $destination && -f $destination ]] || return 1
  fi

  directory="$(dirname -- "$destination")"
  temporary="$(mktemp "$directory/.unit.write.XXXXXX")"
  ready="$(mktemp "$directory/.unit.ready.XXXXXX")"
  if ! cp -- "$source" "$temporary" ||
    ! install -m "$mode" "$temporary" "$ready" ||
    ! mv -fT -- "$ready" "$destination"; then
    rm -f -- "$temporary" "$ready"
    return 1
  fi
  rm -f -- "$temporary"
}

unit_test_root="$(mktemp -d "${TMPDIR:-/tmp}/lumenforge-unit-test.XXXXXX")"
trap 'rm -rf -- "$documentation_test_root" "$unit_test_root"' EXIT
source_unit="$unit_test_root/source.service"
destination_unit="$unit_test_root/LumenForge.service"
printf '[Service]\nExecStart=/bin/true\n' >"$source_unit"

replace_unit_for_test "$source_unit" "$destination_unit" 0644 ||
  fail "temporary-directory unit replacement failed"
[[ $(stat -c '%a' "$destination_unit") == 644 ]] ||
  fail "normal unit replacement did not produce mode 0644"

symlink_target="$unit_test_root/symlink-target"
printf 'unchanged\n' >"$symlink_target"
rm -f -- "$destination_unit"
ln -s -- "$symlink_target" "$destination_unit"
if replace_unit_for_test "$source_unit" "$destination_unit" 0600; then
  fail "temporary-directory unit replacement accepted a symlink destination"
fi
[[ $(<"$symlink_target") == unchanged ]] ||
  fail "symlink rejection changed the symlink target"

rm -f -- "$destination_unit"
printf 'unsafe\n' >"$destination_unit"
chmod 0666 "$destination_unit"
replace_unit_for_test "$source_unit" "$destination_unit" 0600 ||
  fail "replacement of an unsafe-mode regular unit failed"
[[ $(stat -c '%a' "$destination_unit") == 600 ]] ||
  fail "replacement did not correct an unsafe pre-existing mode to 0600"

echo "installer static checks passed"
