# Motherboard PWM Support Bring-up and Validation

This guide is for LumenForge maintainers, advanced contributors adding support
for a motherboard, and users deliberately validating an existing motherboard
definition. It is not required for ordinary use of an already supported board,
and it is not the first troubleshooting step to follow merely because fans do
not appear in the dashboard.

Incorrect PWM mappings can control the wrong physical header or interfere with
expected cooling. Before testing:

- retain a working BIOS fan curve;
- know how to return every tested header to BIOS control;
- identify which fan or pump is connected to each header; and
- keep the system at light load. Do not test under heavy CPU or GPU load.

If cooling behaves unexpectedly, return the affected header to BIOS mode
immediately. If that is not possible in the dashboard, stop LumenForge and
restore control through the motherboard firmware.

## Currently defined motherboards

Only exact boards with definitions in LumenForge are currently expected to
work:

| Motherboard definition name | Chip | Driver (out-of-tree) |
| --- | --- | --- |
| X870E AORUS ELITE WIFI7 | `it8696` | <https://github.com/frankcrawford/it87> |
| X570 AORUS MASTER | `it8688` | <https://github.com/frankcrawford/it87> |
| MAG X670E TOMAHAWK WIFI (MS-7E12) | `nct6687` | <https://github.com/Fred78290/nct6687d> |

Sharing the same Super I/O chip does not make another motherboard compatible.
Each board name and every fan-header mapping must be validated individually.

## Choose the applicable path

- Follow [Path A](#path-a-use-or-validate-an-existing-definition) only when
  `board_name` exactly matches a definition above.
- Follow [Path B](#path-b-add-support-for-a-new-motherboard) when developing a
  definition for another board.

## Path A: use or validate an existing definition

### 1. Confirm the exact board name

```bash
cat /sys/class/dmi/id/board_name
```

The output must exactly match a `name` in LumenForge's
`database/motherboard/motherboard.json`. A similar marketing name is not enough.

### 2. Confirm the driver and hwmon device

Install and load only the driver documented for the defined board. Driver
installation is outside this guide; follow that driver's documentation.

List hwmon devices by their stable reported names:

```bash
for file in /sys/class/hwmon/hwmon*/name; do
  printf '%s=%s\n' \
    "$(basename "$(dirname "$file")")" \
    "$(cat "$file")"
done
```

Representative output from one machine:

```text
hwmon0=acpitz
hwmon1=it8696
hwmon5=amdgpu
hwmon7=k10temp
```

Here `hwmon1` happens to report `it8696`. Names such as `hwmon1` are assigned
dynamically and may change between boots. LumenForge finds the device by the
contents of its `name` file, so do not store or rely on `hwmon1` as a permanent
identifier.

Confirm that the expected input, mode, and PWM files exist beneath the matching
hwmon directory:

```bash
find -L /sys/class/hwmon/hwmon1 -maxdepth 1 -type f \
  \( -name 'fan*_input' -o -name 'pwm*' \) -printf '%f\n'
```

`hwmon1` in this command is an example. Replace it with the current directory
that reports the expected chip. Representative files are:

```text
fan1_input
pwm1
pwm1_enable
fan2_input
pwm2
pwm2_enable
```

The three files for a header have different roles:

- `fan1_input` reports the fan's RPM.
- `pwm1_enable` selects the control mode.
- `pwm1` sets the PWM value.

For the currently defined boards, mode `1` is PWM control and mode `2` is BIOS
control:

```bash
cat /sys/class/hwmon/hwmon1/pwm1_enable
```

Example output:

```text
2
```

Replace both occurrences of `hwmon1` with the current matching hwmon directory.

### 3. Grant narrowly scoped PWM access

The rule below matches one reported chip name, changes the `pwm*` files to the
`lumenforge` group, and grants that group read/write access. It does not grant
write access to every hwmon device. Replace `it8696` with the exact chip from
the existing motherboard definition:

```bash
sudo tee /etc/udev/rules.d/99-motherboard-pwm.rules >/dev/null <<'EOF'
SUBSYSTEM=="hwmon", KERNEL=="hwmon*", ATTR{name}=="it8696", RUN+="/bin/sh -c 'chgrp -h lumenforge /sys%p/pwm* 2>/dev/null || true; chmod g+rw /sys%p/pwm* 2>/dev/null || true'"
EOF
```

The same group rule serves both provided installation modes. The
user-service installer adds the desktop user to the `lumenforge` group; the
system service runs with `lumenforge` as its primary group. If using a
nonstandard service identity, do not broaden the rule: adapt the group only
after confirming that installation model.

Reload the rule and retrigger hwmon devices:

```bash
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hwmon
```

### 4. Enable the documented configuration

In the applicable `config.json`, set:

```json
"enableMotherboard": true,
"motherboardBiosOnExit": true,
```

`motherboardBiosOnExit` tells LumenForge to return managed headers to the mode
labelled `BIOS` when the device stops. Restart the selected service after
editing:

```bash
# User service
systemctl --user restart LumenForge.service

# System service
sudo systemctl restart LumenForge.service
```

### 5. Validate conservatively

1. Keep the machine at light load.
2. Test one header at a time in the dashboard.
3. Visually or physically identify the fan connected to that header.
4. Make a small, conservative speed change and confirm that the corresponding
   RPM reading responds.
5. Return the header to BIOS mode before moving to the next header.

If the wrong fan responds, the RPM does not correspond to the connected fan, or
cooling becomes unexpected, return the header to BIOS mode immediately and stop
testing. Stop LumenForge if needed:

```bash
# User service
systemctl --user stop LumenForge.service

# System service
sudo systemctl stop LumenForge.service
```

Restore the working BIOS fan configuration before resuming normal load.

## Path B: add support for a new motherboard

This is development work. Work in a source checkout and edit
`database/motherboard/motherboard.json` there. Installed files beneath
`/opt/LumenForge` are immutable and must not be edited.

### 1. Collect the board and hwmon facts

Record:

- the exact value from `/sys/class/dmi/id/board_name`;
- the Super I/O chip and required kernel driver;
- the hwmon directory currently reporting that chip;
- every `fanX_input`, `pwmX_enable`, and `pwmX` file;
- the meaning of each supported mode value; and
- the physical fan or pump attached to each mapped header.

Use the commands in Path A to inspect these values. Remember that the current
`hwmonN` directory is diagnostic information, not a stable identifier.

### 2. Define the board and headers

The top-level `entry` is the hwmon directory LumenForge scans. Each motherboard
definition contains:

- `name`: the exact DMI board name used for matching;
- `displayName`: the human-readable dashboard name;
- `chip`: the exact value expected in the hwmon `name` file;
- `interval`: the telemetry refresh interval in milliseconds; and
- `headers`: the individually validated physical header mappings.

Each header contains:

- `id`: its numeric LumenForge channel identifier;
- `headerName`: the dashboard label;
- `headerInput`: the RPM input file, such as `fan1_input`;
- `headerConfig`: the mode file, such as `pwm1_enable`;
- `headerLabel`: an optional driver-provided label file;
- `headerModes`: the driver's numeric mode values and their meanings; and
- `headerValue`: the PWM output file, such as `pwm1`; LumenForge converts the
  selected percentage to the driver's raw `0`-through-`255` scale.

`headerInput`, `headerConfig`, and `headerValue` must all describe the same
physical header. Never infer that correspondence only because the filenames
share a number; validate the physical fan and RPM response.

For the current definitions, `headerModes` maps `1` to `PWM` and `2` to `BIOS`.
The current parser does not use a `defaultHeaderMode` field. It reads the
header's current mode and identifies BIOS control from the `BIOS` label.

A minimal one-header development example is:

```json
{
  "name": "EXACT DMI BOARD NAME",
  "displayName": "Human-readable board name",
  "chip": "reported_hwmon_chip",
  "interval": 3000,
  "headers": {
    "1": {
      "id": 1,
      "headerName": "Fan 1",
      "headerInput": "fan1_input",
      "headerConfig": "pwm1_enable",
      "headerModes": {
        "1": "PWM",
        "2": "BIOS"
      },
      "headerValue": "pwm1"
    }
  }
}
```

All names in this example are placeholders except the currently supported mode
mapping. Use only fields and mappings verified for the new board.

### 3. Validate and contribute

Apply the narrow udev rule for the verified chip, enable motherboard support in
the development configuration, and validate each header individually using the
conservative procedure in Path A. Do not add an untested header to the
definition.

After validating the complete board mapping, submit the source change to
`database/motherboard/motherboard.json` through a pull request, including the
exact board name, chip, driver, and validation performed.
