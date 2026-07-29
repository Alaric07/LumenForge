# Memory DDR4 / DDR5 Configuration

Enabling memory support lets LumenForge discover and control supported Corsair
DDR4 or DDR5 modules, including RGB and temperature reporting where the module
and kernel drivers provide those features.

This is advanced hardware setup. LumenForge opens the configured motherboard
SMBus/I2C device directly. Bus numbers and the examples in this guide are
machine-specific. Stop if you cannot confidently identify the motherboard
SMBus; leaving memory support disabled is safer than guessing.

## Prerequisites

Before changing the configuration, confirm that you have:

- a memory family listed in the [supported device list](supported-devices.md);
- `i2c-tools`, which provides `i2cdetect`;
- LumenForge installed as either a user-service installation or a
  system-service installation;
- administrator access for the diagnostic and udev commands; and
- a current copy of the applicable `config.json`.

For a user-service installation, back up the configuration with:

```bash
cp "${XDG_CONFIG_HOME:-$HOME/.config}/lumenforge/config.json" \
  "${XDG_CONFIG_HOME:-$HOME/.config}/lumenforge/config.json.before-memory"
```

For a system-service installation, use:

```bash
sudo cp /var/lib/lumenforge/config.json \
  /var/lib/lumenforge/config.json.before-memory
```

### Install i2c-tools

These are the existing package examples for Fedora and Debian:

```bash
# Fedora
sudo dnf install i2c-tools

# Debian
sudo apt install i2c-tools
```

If your distribution uses another package manager, install its package that
provides `i2cdetect`.

## Identify the motherboard SMBus

List the available I2C adapters:

```bash
sudo i2cdetect -l
```

The output below is from one example machine. Its motherboard SMBus adapters
are `i2c-15`, `i2c-16`, and `i2c-17`; those numbers are not instructions for
another machine.

```text
i2c-0   i2c             Synopsys DesignWare I2C adapter         I2C adapter
i2c-2   i2c             NVIDIA i2c adapter 1 at 1:00.0          I2C adapter
i2c-9   i2c             AMDGPU DM i2c hw bus 0                  I2C adapter
i2c-15  smbus           SMBus PIIX4 adapter port 0 at 0b00      SMBus adapter
i2c-16  smbus           SMBus PIIX4 adapter port 2 at 0b00      SMBus adapter
i2c-17  smbus           SMBus PIIX4 adapter port 1 at 0b20      SMBus adapter
```

Do not assume the first adapter labelled `smbus` is correct. Use the adapter
description, motherboard and kernel-driver information, and known DIMM devices
to identify the motherboard bus. Do not select GPU, display, or other unrelated
I2C adapters. If the available information does not identify the correct bus,
stop here.

### Advanced diagnostic: probe the selected bus

`i2cdetect -y` probes addresses on the selected bus. Run it only after
identifying a likely motherboard SMBus, and do not scan unrelated adapters.
Replace the value below with the numeric part of that bus name before running:

```bash
BUS_NUMBER='replace-with-numeric-bus'
sudo i2cdetect -y "$BUS_NUMBER"
```

For example, the following command and output apply only to the example
machine's `i2c-15` bus:

```bash
sudo i2cdetect -y 15
```

```text
     0  1  2  3  4  5  6  7  8  9  a  b  c  d  e  f
00:                         -- -- -- -- -- -- -- --
10: -- -- -- -- -- -- -- -- -- 19 -- 1b -- -- -- --
20: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
30: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
40: -- -- -- -- -- -- -- -- -- 49 -- 4b -- -- -- --
50: -- UU -- UU -- -- -- -- -- -- -- -- -- -- -- --
60: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
70: -- -- -- -- -- -- -- --
```

Memory-related devices often appear around the SPD EEPROM address range
beginning at `0x50`; `UU` means a kernel driver has already claimed that
address. This is useful supporting evidence, but no single output pattern proves
that a bus or memory kit is compatible with LumenForge. Stop rather than trying
other buses at random.

## Find the memory part number

LumenForge attempts to decode the DDR5 SKU automatically when the supported
`spd5118` EEPROM path is available. DDR4 does not use that DDR5 decoder, so a
DDR4 setup normally needs the exact Corsair part number in `memorySku`. For
DDR5, set `memorySku` only when automatic decoding produces no usable value.

Record the part number reported by the system:

```bash
sudo dmidecode -t memory | grep 'Part Number'
```

Example output:

```text
        Part Number: CMT64GX5M2B5600Z40
        Part Number: CMT64GX5M2B5600Z40
```

## Configure LumenForge

Edit the same `config.json` that you backed up. Set:

- `memory` to `true`;
- `memorySmBus` to the exact verified device basename, such as `i2c-15`;
- `memoryType` to `4` for DDR4 or `5` for DDR5;
- `memorySku` to the exact part number only when the fallback is needed; and
- `ramTempViaHwmon` according to whether the supported DIMM hwmon path is
  available.

This example is machine-specific:

```json
"memory": true,
"memorySmBus": "i2c-15",
"memoryType": 5,
"memorySku": "CMT64GX5M2B5600Z40",
"ramTempViaHwmon": true,
```

LumenForge does not implement a `decodeMemorySku` configuration field, alias,
command-line flag, or environment-variable override. The current
[OpenLinkHub README](https://github.com/jurkovic-nikola/OpenLinkHub#6-configuration)
may still mention that option, but it is not a LumenForge setting.

## Grant access to the selected bus

The custom udev rule grants LumenForge access only to the named I2C device.
Replace every `i2c-15` below with the verified device basename for your
machine. The user-service rule grants read/write access to members of the
`lumenforge` group. The system-service rule grants read/write access only to
the dedicated `lumenforge` account.

For a user-service installation:

```bash
echo 'KERNEL=="i2c-15", MODE="0660", GROUP="lumenforge"' | sudo tee /etc/udev/rules.d/98-corsair-memory.rules
sudo udevadm control --reload-rules
sudo udevadm trigger
```

For a system-service installation:

```bash
echo 'KERNEL=="i2c-15", MODE="0600", OWNER="lumenforge"' | sudo tee /etc/udev/rules.d/98-corsair-memory.rules
sudo udevadm control --reload-rules
sudo udevadm trigger
```

## Restart and verify

After a first user-service installation, reboot before testing memory access if
the installer added the desktop user to the `lumenforge` group. A complete
logout and login can refresh supplementary groups, but reboot is the
recommended and tested procedure. Restarting only the service does not update
group membership in the existing login session.

For later user-service configuration changes, when the current session already
has the group membership:

```bash
systemctl --user restart LumenForge.service
systemctl --user status LumenForge.service
```

For the system service:

```bash
sudo systemctl restart LumenForge.service
sudo systemctl status LumenForge.service
```

Success means the service is active and the supported memory appears in the
dashboard. If it does not, inspect the applicable logs:

```bash
# User service
journalctl --user -u LumenForge.service -n 100 --no-pager

# System service
sudo journalctl -u LumenForge.service -n 100 --no-pager
```

Do not try unverified buses merely to make memory appear.

## Rollback

1. Set `"memory": false` in `config.json`, or restore the backup made before
   setup.
2. Remove or disable `/etc/udev/rules.d/98-corsair-memory.rules` if it is no
   longer needed, then reload the udev rules.
3. If you manually added a kernel parameter while troubleshooting, remove it
   through your distribution's normal bootloader configuration.
4. Restart the applicable service, or reboot if group membership or the kernel
   command line changed.

## Advanced troubleshooting: firmware resource conflicts

Some systems do not expose the motherboard SMBus because the kernel protects
firmware-declared hardware resources. The kernel parameter
`acpi_enforce_resources=lax` changes how those resources are handled. It should
not be added casually, and it does not guarantee memory support.

If you have confirmed that a firmware resource conflict is the cause, consult
your distribution's documentation for safely adding and removing kernel
parameters. Reboot after the change, identify the bus again, and do not assume
its number matches an earlier example. Remove the parameter through the same
distribution-supported process if it causes problems.
