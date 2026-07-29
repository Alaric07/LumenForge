# Troubleshooting

## Start here

LumenForge supports one active service mode at a time. Determine whether you
installed the user service or system service, then use only the commands for
that mode. Preserve the applicable configuration and mutable-data directories
before reinstalling, switching service modes, or attempting destructive
recovery.

The dashboard is local-only and is normally available from the same machine at
<http://127.0.0.1:27003>.

| Service mode | Status | Restart | Recent journal |
| --- | --- | --- | --- |
| User service | `systemctl --user status LumenForge.service` | `systemctl --user restart LumenForge.service` | `journalctl --user -u LumenForge.service -n 100 --no-pager` |
| System service | `sudo systemctl status LumenForge.service` | `sudo systemctl restart LumenForge.service` | `sudo journalctl -u LumenForge.service -n 100 --no-pager` |

`active (running)` means systemd has a running LumenForge process. It does not
by itself prove that every device, sensor, or integration initialized.

Installed paths depend on the service mode:

| Service mode | Configuration | Mutable data |
| --- | --- | --- |
| User service | `$XDG_CONFIG_HOME/lumenforge/config.json`, or `~/.config/lumenforge/config.json` when `XDG_CONFIG_HOME` is unset | `$XDG_DATA_HOME/lumenforge`, or `~/.local/share/lumenforge` when `XDG_DATA_HOME` is unset |
| System service | `/var/lib/lumenforge/config.json` | `/var/lib/lumenforge` |

See [Filesystem Layout and Ownership](filesystem-layout.md) for the complete
installed layout.

## The service does not start

1. Run the matching status command from the table above.
2. Read the matching recent journal and start with the first relevant error.
   Later errors may only be consequences of the first one.
3. Confirm that only one service mode is installed or active. The installers
   reject detected conflicts; do not try to run both modes.
4. Confirm that the installed tree and executable exist:

   ```bash
   ls -ld /opt/LumenForge /opt/LumenForge/LumenForge
   ```

5. Check the correct `config.json`. If the optional `jq` utility is already
   installed, validate its syntax:

   ```bash
   jq empty "${XDG_CONFIG_HOME:-$HOME/.config}/lumenforge/config.json"
   sudo jq empty /var/lib/lumenforge/config.json
   ```

   Run only the line for the installed service mode. No output means `jq`
   accepted the JSON syntax; it does not validate every LumenForge value.
6. Check whether another local process is already listening on the configured
   dashboard port. Replace `27003` if `listenPort` is different:

   ```bash
   ss -ltnp 'sport = :27003'
   ```

7. Do not repeatedly reinstall before preserving state and understanding the
   error. Reinstallation can hide the original diagnostic without correcting
   configuration, permission, or port problems.

## The dashboard does not open

- Open `http://127.0.0.1:27003` or `http://localhost:27003` on the same machine
  that runs LumenForge. Use the configured `listenPort` if it is not `27003`.
- Confirm that the matching service reports `active (running)`.
- Check the port with the `ss` command above.
- LumenForge listens only on IPv4 loopback. Remote browsers, LAN or Tailscale
  addresses, reverse proxies, and wildcard listeners are unsupported.
- Request Host validation accepts only `127.0.0.1` or `localhost` with the
  configured port. An arbitrary hostname or LAN address will not work.
- After upgrading frontend files or changing the External Source Registry, use
  a browser hard refresh before assuming that a stale page reflects current
  state.

Do not weaken the local-only protections to work around a dashboard problem.

## Expected devices are missing

1. Check the [native supported-device list](supported-devices.md), including the
   connected device's PID where applicable. OpenRGB-backed devices follow the
   separate [OpenRGB import](openrgb-import.md) workflow.
2. If available, check what Linux can see:

   ```bash
   lsusb
   ```

   Compare the product part of the `vendor:product` pair with the PID table.
   Device names alone do not guarantee every firmware, connection mode, or
   feature.
3. Check the matching service journal for discovery, permission, or device-open
   errors.
4. After a first user-service installation that changed group membership or
   udev access, follow the installer's reboot instruction. Restarting only the
   service does not refresh the current login session's groups.
5. Close another RGB or hardware-control application if it may be claiming the
   same device, then restart only the matching LumenForge service.
6. For a Slipstream-connected device, follow the
   [Slipstream pairing note](supported-devices.md#slipstream-pairing).

Use the dedicated setup guide when the device requires additional work:

- [Memory DDR4 / DDR5](memory-configuration.md)
- [Motherboard PWM](motherboard-pwm.md)
- [SCUF controller audio configuration](scuf-controller.md)
- [XENEON EDGE KDE](xeneon-edge-kde.md)

## Permission errors

The user-service installer grants device access through the `lumenforge` group
and shipped udev rules. If it added your account to that group, reboot before
relying on hardware access. A complete logout and login can refresh group
membership, but restarting only `LumenForge.service` cannot change the groups
of the existing login session.

Inspect instead of applying broad permissions:

```bash
id
ls -l /etc/udev/rules.d/99-lumenforge.rules
ls -l /dev/hidraw*
```

Missing paths may be normal for hardware that is not connected. Use the shipped
installer and documented udev rule to correct installation issues. Do not make
devices or state world-writable, and do not run the user service as root as a
workaround.

## OpenRGB import problems

See [OpenRGB Device Import](openrgb-import.md) for the complete workflow. Check
that:

- OpenRGB is already running; LumenForge does not start or configure it.
- OpenRGB's **SDK Server** is enabled on `127.0.0.1`.
- its port matches LumenForge's `openRGBPort` (default `6742`);
- no remote OpenRGB host is being used;
- **Settings** > **OpenRGB SDK Integration** > **Discover & Manage
  Controllers** can discover the controller;
- **Import Selected** was used for a new controller; and
- **Discover Again** or **Refresh Imported Controllers** was used for the
  intended operation.

Discovery and import are separate. OpenRGB recognition also does not guarantee
that its SDK metadata is sufficient for every LumenForge feature.

## External Sources do not appear or update

See [External Source Registry](external-sources.md). The registry is loaded and
validated on demand, so a service restart is not normally required after saving
it.

- Hard-refresh **Cooling Profiles** so the dropdown requests the registry
  again.
- Check the registry path for the active service mode and inspect its ownership
  and permissions.
- Check the journal for the configured source ID and for registry, executable,
  output, or timeout errors.
- Confirm that the saved profile still selects an ID present in the registry.
- Use a service restart only as an optional troubleshooting step, not as the
  normal reload mechanism.

## Cooling profile problems

See [Cooling Profiles and Fan Curves](cooling-profiles.md). Confirm that:

- the profile was saved;
- it was assigned to the intended device or channel;
- the correct **Sensor** and any dependent source were selected;
- a current temperature value is available;
- the channel supports software cooling control; and
- actual RPM and temperature response were observed.

If `manual` is `true` in `config.json`, LumenForge does not run automatic
cooling curves and rejects profile assignment. Set it back to `false` only if
automatic control is intended, then restart the matching service. If a sensor
is missing or behavior is uncertain, return the channel to its previous or
**Normal** profile.

## Configuration changes do not take effect

LumenForge reads `config.json` during startup. Follow the
[Configuration Reference](configuration.md), edit the path for the installed
service mode, validate the JSON, and restart that same service.

Not every dashboard-managed value belongs to `config.json`. Device profiles,
cooling profiles, RGB state, imported-controller records, and other generated
state are stored below the mutable data root. Do not edit `/opt/LumenForge` as
runtime state.

## Backup or restore problems

See [Backup and Restore](backup-restore.md) for accepted contents and limits.

- The uploaded file must be a backup created by LumenForge.
- Restore validates the complete archive before replacing live state.
- After a successful restore, make no further dashboard changes and restart the
  matching service immediately.
- Check the journal for validation, staging, commit, or rollback errors.
- Preserve a separate, out-of-band copy of current state before testing
  restore.

## Upgrade or reinstall problems

- Use a fresh source checkout for the current alpha.
- Build and run the same installer mode as the existing installation.
- Never run either installer from `/opt/LumenForge`.
- Preserve configuration and mutable data before changing service modes.
- Do not install or run user and system service modes at the same time.

Return to the README for the current [upgrade](../README.md#upgrades) and
[uninstall](../README.md#uninstall) procedures.

## Collecting useful information for a bug report

Provide only the information needed to reproduce the problem:

- LumenForge version;
- Linux distribution and desktop/session type;
- user or system service mode;
- affected device model and PID;
- a relevant journal excerpt around the failure;
- clear reproduction steps;
- expected and actual behavior; and
- whether the problem remains after a service restart or reboot.

Review logs and configuration before posting them. Remove usernames and home
paths when they are not relevant, hardware serial numbers, IP addresses,
External Source commands or sensitive arguments, and other private system
information. Do not upload an entire configuration or complete journal by
default.
