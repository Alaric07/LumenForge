# SCUF Controller Audio Configuration

Use this guide only if audio routed through a SCUF Envision controller is
noticeably quieter than expected and the device's ALSA mixer control is set to
a reduced value. The workflow changes ALSA mixer state and, optionally,
WirePlumber policy. Confirm the problem with both software and physical volume
set conservatively before changing anything.

Cable and wireless-dongle connections appear as different ALSA and PipeWire
devices. Discover and configure each connection separately.

## Prerequisites

The commands use:

- `cat`, `grep`, and `sed`;
- `amixer` and `alsactl` from the ALSA utilities;
- `pactl` from the PulseAudio-compatible client utilities provided by the
  active audio stack; and
- WirePlumber and PipeWire for the optional software-volume policy.

This guide does not assume a package manager. Install the packages that provide
those tools through your distribution if a command is missing.

## Identify the SCUF ALSA card

Connect only the cable or wireless dongle you intend to configure, then list
the ALSA cards:

```bash
cat /proc/asound/cards
```

A cabled controller may resemble:

```text
5 [V2             ]: USB-Audio - SCUF Envision Pro Controller V2
```

A wireless dongle may resemble:

```text
5 [USB            ]: USB-Audio - SCUF Envision Pro Wireless USB
```

These are examples from one machine. Card number `5` and short identifiers
`V2` and `USB` are not universal and can change when devices reconnect or the
machine reboots. Record both the current card number and the short identifier
shown in brackets for the connection being configured.

Set the following shell variables to those discovered values before using the
commands below:

```bash
ALSA_CARD_NUMBER='replace-with-card-number'
ALSA_CARD_ID='replace-with-short-identifier'
```

## Inspect and record the mixer control

List the controls on the selected card:

```bash
amixer -c "$ALSA_CARD_NUMBER" controls
amixer -c "$ALSA_CARD_NUMBER" contents
```

Identify the playback control associated with the SCUF audio output from its
name, range, and current values. Do not assume that `numid=8` belongs to that
control on every controller revision.

After identifying the control, set its number and inspect the current value:

```bash
CONTROL_NUMID='replace-with-control-numid'
amixer -c "$ALSA_CARD_NUMBER" cget "numid=$CONTROL_NUMID"
```

Record the complete output for rollback before changing it. If the control
cannot be identified confidently, stop here.

## Change and persist the mixer value

The value `32,32` corrected reduced output on the original tested setup, where
the same control reported `16,16`. It is an example, not a universal value.
Confirm the selected control's allowed range before using it or choose the
appropriate verified value for your device:

```bash
MIXER_VALUE='replace-with-verified-value'
sudo amixer -c "$ALSA_CARD_NUMBER" \
  cset "numid=$CONTROL_NUMID" "$MIXER_VALUE"
```

The equivalent command using the short ALSA identifier is:

```bash
sudo amixer -D "hw:$ALSA_CARD_ID" \
  cset "numid=$CONTROL_NUMID" "$MIXER_VALUE"
```

Use either the card-number form or the identifier form, not both. Re-run the
matching `cget` command and verify that the intended control changed.

After verifying the result, persist the current ALSA state for this card:

```bash
sudo alsactl store "$ALSA_CARD_NUMBER"
```

`alsactl store` writes the current driver mixer state to the system ALSA state
file so it can be restored later. It persists more than the single value shown
here, which is why the state should be stored only after verification.

## Optional: use software volume through WirePlumber

Use this section only if hardware mixer behavior still prevents normal software
volume control. The policy disables ACP handling for the exact SCUF device and
uses PipeWire's software mixer instead.

### Discover the PipeWire device name

List PipeWire/PulseAudio-compatible card information:

```bash
pactl list cards
```

Find the block describing the connected SCUF cable or dongle and record its
exact `device.name`. A value can resemble:

```text
device.name = "alsa_card.usb-Scuf_Gaming_SCUF_Envision_Pro_Wireless_USB_Receiver_V2_1c629ed800020217-00"
```

The complete value, including its serial-specific portion, is
machine-specific. Do not copy the example. Older dongles may be labelled
`SCUF PC Controller`; identify them from their own card block.

### Create a connection-specific policy file

Use separate filenames so cable and wireless policies can be removed
independently:

- `94-scuf-wireless-no-acp.conf` for the wireless dongle;
- `95-scuf-cable-no-acp.conf` for the cabled controller.

Create the directory:

```bash
mkdir -p ~/.config/wireplumber/wireplumber.conf.d
```

For the wireless dongle, replace `<WIREPLUMBER_DEVICE_NAME>` with the exact
discovered `device.name` before running:

```bash
cat > ~/.config/wireplumber/wireplumber.conf.d/94-scuf-wireless-no-acp.conf <<'EOF'
monitor.alsa.rules = [
  {
    matches = [
      { device.name = "<WIREPLUMBER_DEVICE_NAME>" }
    ]
    actions = {
      update-props = {
        api.alsa.use-acp = false
        api.acp.auto-profile = false
        api.acp.auto-port = false
        api.alsa.soft-mixer = true
        api.alsa.soft-vol = true
        api.alsa.disable-mixer = true
      }
    }
  }
]
EOF
```

For a cable policy, use the same content with the cable's exact device name and
write it to:

```text
~/.config/wireplumber/wireplumber.conf.d/95-scuf-cable-no-acp.conf
```

Restart the user audio services to load the policy:

```bash
systemctl --user restart wireplumber pipewire pipewire-pulse
```

## Verify

1. Reconnect the applicable cable or dongle if it is not detected after the
   service restart.
2. Start audio at a conservative physical and software volume.
3. Confirm that output is present on the intended SCUF device.
4. Adjust software volume gradually and confirm that it behaves normally.
5. Repeat discovery from the beginning before configuring the other connection.

## Rollback

Restore the mixer control to the value recorded before the change:

```bash
MIXER_VALUE='replace-with-recorded-original-value'
sudo amixer -c "$ALSA_CARD_NUMBER" \
  cset "numid=$CONTROL_NUMID" "$MIXER_VALUE"
sudo alsactl store "$ALSA_CARD_NUMBER"
```

Remove only the WirePlumber file created for the affected connection:

```bash
rm -f ~/.config/wireplumber/wireplumber.conf.d/94-scuf-wireless-no-acp.conf
rm -f ~/.config/wireplumber/wireplumber.conf.d/95-scuf-cable-no-acp.conf
```

If only one file was created, remove only that file. Then restart the user
audio services:

```bash
systemctl --user restart wireplumber pipewire pipewire-pulse
```
