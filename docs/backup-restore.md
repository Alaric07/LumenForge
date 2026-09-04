# Backup and Restore

LumenForge's Settings page downloads and restores a ZIP snapshot of its
configuration and mutable runtime state. Restore is an alpha feature: make a
separate copy of important state before testing it, and treat every uploaded
backup as untrusted input.

## Create a backup

1. Open the LumenForge dashboard and select **Settings**.
2. Find **Backup and Restore** and select **Backup**.
3. Wait for the browser to finish downloading the ZIP file, then store it
   somewhere separate from the LumenForge installation.

The **Backup** button always creates one complete snapshot of all supported
LumenForge configuration and mutable state that currently exists. Users cannot
select individual backup components. Creating a backup does not require a
service restart.

## Restore a backup

1. Before restoring, create a new backup or separately copy any current state
   you may need.
2. In **Admin**, find **Backup and Restore** and select the LumenForge backup
   ZIP file.
3. Select **Restore**.
4. When the dashboard reports success, make no further dashboard changes and
   restart LumenForge immediately.

A successful restore response means the files were replaced; it does not mean
the running process reloaded them. Restart the service mode used by the
installation:

```bash
# User service
systemctl --user restart LumenForge.service

# System service
sudo systemctl restart LumenForge.service
```

After the restart, LumenForge loads the restored configuration, profiles, and
mutable state. Confirm that the dashboard shows the expected restored state and
currently available hardware. If the service does not start or the restored
state is not as expected, inspect the service status and logs described under
[Troubleshooting](#troubleshooting).

## What the snapshot contains and replaces

A generated LumenForge backup contains:

```text
config.json                 always included
database/                   mutable database snapshot
database/**                 all existing files and subdirectories
dashboard.json              included automatically when present
display.json                included automatically when present
_hash.txt                   always included
```

`dashboard.json` and `display.json` do not exist on every installation. The
**Backup** button includes each file automatically when it is present; this is
not a user-selectable option.

Restore uses snapshot semantics:

- `database/` replaces the complete live mutable database. Files present only
  in the live database, including files created after the backup, are removed.
- A present `dashboard.json` or `display.json` replaces the corresponding live
  file.
- If `dashboard.json` or `display.json` is absent from the backup, its live copy
  is removed.
- The archived `logFile` and `amdsmiPath` values never replace the current
  host's values. All other archived configuration fields, including unknown
  fields, are retained.

External Source Registry files are not part of this snapshot and are never
backed up or restored.

## Technical details

### Valid archive format and limits

`config.json` and `_hash.txt` must each appear exactly once in a valid backup.
Directories are accepted only beneath `database/`. Every other entry must be a
regular file at one of the paths shown above. Absolute paths, traversal,
backslashes, non-canonical or duplicate paths, symbolic links, devices, FIFOs,
sockets, and other special entries are rejected.

The compressed HTTP upload limit is 5 MiB. After opening the ZIP, restore also
enforces these uncompressed limits:

| Limit | Maximum |
| --- | ---: |
| Archive entries, including directories | 4,096 |
| Path depth | 16 components |
| One regular file | 32 MiB |
| All restored regular files combined | 128 MiB |
| `_hash.txt` | 128 bytes |

Both ZIP metadata and the bytes produced by decompression are bounded. These
limits are intentionally above LumenForge's current profiles and 5 MiB media
upload limit while preventing a small compressed upload from expanding without
bound.

### Integrity validation

`_hash.txt` is the SHA-256 digest of the concatenated regular-file contents in
archive order, excluding `_hash.txt` itself. It detects accidental corruption;
it is not a signature and does not prove who created the backup. Restore also
checks ZIP decompression and CRC errors and validates the JSON syntax of
`config.json`, `dashboard.json`, `display.json`, and every `.json` file beneath
`database/`. Non-JSON mutable files, such as LCD media, remain supported.

### Restore staging, commit, and rollback

Restore validates the complete archive before creating private staging
directories beside the configured destinations. Staging directories use mode
`0700`; staged files use mode `0600`. Nothing at a live restore target is
changed until structure, limits, the corruption hash, decompression, and JSON
syntax have all passed.

At commit time LumenForge briefly renames each current target to a unique
sibling name, renames the staged replacement into place, and then removes the
temporary originals after all targets have been installed. If a later rename
fails, it attempts to put every original back. A validation or staging failure
therefore leaves live state unchanged, and ordinary commit failures roll back.

This is a small local rename transaction, not a filesystem snapshot. A sudden
power loss during the bounded rename sequence can still leave an intermediate
state, and a filesystem failure can prevent rollback. Check the local service
log if restore reports a commit or rollback failure.

### Restore paths and runtime coordination

Installed user services restore `config.json` beneath
`$XDG_CONFIG_HOME/lumenforge/` (falling back to `~/.config/lumenforge/`) and
mutable data beneath `$XDG_DATA_HOME/lumenforge/` (falling back to
`~/.local/share/lumenforge/`). The system service restores both beneath
`/var/lib/lumenforge/`. Restore never targets `/opt/LumenForge` or
`/etc/lumenforge`.

A direct development run still uses its working directory for configuration
and mutable data, but restore accepts only `config.json`, `database/**`,
`dashboard.json`, and `display.json`. It cannot restore source files, static
assets, templates, `go.mod`, installer scripts, or other application content.

Backup creation and restore are serialized with each other. Ordinary runtime
profile or device writes are not globally paused, so restart immediately after
a successful restore and do not make further dashboard changes first.

## Troubleshooting

Inspect the user-service log and status:

```bash
systemctl --user status LumenForge.service
journalctl --user -u LumenForge.service -n 100 --no-pager
```

Inspect the system-service log and status:

```bash
sudo systemctl status LumenForge.service
sudo journalctl -u LumenForge.service -n 100 --no-pager
```

If restore rejects a backup, keep the current live data in place. Do not edit
`_hash.txt` to force acceptance: recreate the backup from a known-good running
installation or inspect the ZIP offline for the reported structure, size,
corruption, or JSON problem.
