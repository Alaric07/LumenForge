# XENEON EDGE Touch Mapping on KDE Plasma

In a multi-monitor KDE Plasma session, touch input from the XENEON EDGE may be
mapped to the wrong display. This guide maps that touchscreen to the XENEON
EDGE display. It does not document other desktop environments.

## Recommended: KDE System Settings

The graphical method is the safest approach because Plasma discovers the
display and touchscreen identifiers for you.

1. Open **System Settings**.
2. Search for **Touchscreen**, or open the touchscreen page beneath
   **Input Devices**.
3. Select the touchscreen associated with the XENEON EDGE.
4. Change **Target display** from **Automatic** to **XENEON EDGE**.
5. Select **Apply**.

The exact page and control names can vary slightly between Plasma versions.
Success means touching the XENEON EDGE moves or clicks the pointer on that
display rather than another monitor.

## Advanced/manual configuration

Use this method only when the graphical control is unavailable or when
inspecting an existing mapping. Every identifier in the configuration is
machine-specific:

- the KScreen output number;
- the connector name;
- the output UUID;
- both numeric Libinput group names; and
- the touchscreen device name.

Do not copy identifiers from an example or another machine.

### 1. Find the XENEON EDGE output UUID

Run:

```bash
kscreen-doctor -o
```

Find the output whose connector, geometry, and mode identify the XENEON EDGE.
A relevant line can resemble:

```text
Output: 3 HDMI-A-4 ed36747c-ceb1-402c-af08-f963d94bec99
```

In that example, `3` is the current output number, `HDMI-A-4` is the current
connector name, and the long final value is the UUID. All three values are
machine-specific, and only the UUID is written to `kcminputrc`.

### 2. Find the existing Libinput group path

KDE stores per-device input settings in `kcminputrc`. List its existing
Libinput section headers:

```bash
KDE_INPUT_CONFIG="${XDG_CONFIG_HOME:-$HOME/.config}/kcminputrc"
grep -n '^\[Libinput\]' "$KDE_INPUT_CONFIG"
```

A section header has this structure:

```text
[Libinput][<FIRST_NUMERIC_GROUP>][<SECOND_NUMERIC_GROUP>][<TOUCHSCREEN_NAME>]
```

Identify the section whose final group is the XENEON EDGE touchscreen name.
The numeric groups and device name come from KDE's existing configuration; they
are not values that can be derived reliably from the display UUID. If no
matching touchscreen section exists, use System Settings rather than guessing
group names.

### 3. Record and change the mapping

Replace all four values below with the UUID and exact group names discovered on
this machine:

```bash
OUTPUT_UUID='replace-with-output-uuid'
FIRST_NUMERIC_GROUP='replace-with-first-numeric-group'
SECOND_NUMERIC_GROUP='replace-with-second-numeric-group'
TOUCHSCREEN_NAME='replace-with-exact-touchscreen-name'
```

Record the existing mapping before changing it:

```bash
kreadconfig6 \
  --file kcminputrc \
  --group Libinput \
  --group "$FIRST_NUMERIC_GROUP" \
  --group "$SECOND_NUMERIC_GROUP" \
  --group "$TOUCHSCREEN_NAME" \
  --key OutputUuid
```

Write the discovered display UUID and notify the KDE session:

```bash
kwriteconfig6 \
  --file kcminputrc \
  --group Libinput \
  --group "$FIRST_NUMERIC_GROUP" \
  --group "$SECOND_NUMERIC_GROUP" \
  --group "$TOUCHSCREEN_NAME" \
  --key OutputUuid \
  --notify \
  "$OUTPUT_UUID"
```

The notification may apply the change immediately. If it does not, log out of
the KDE session and log back in; no broader system restart is required.

## Verify

Touch several points on the XENEON EDGE at low pointer speed. The pointer
should appear at the corresponding position on that display and should not move
on another monitor.

## Rollback

The recommended rollback is to return to KDE System Settings, set **Target
display** to **Automatic**, and select **Apply**.

For a manual rollback, use the same discovered group path. If an `OutputUuid`
value existed before the change, assign that recorded value in place of
`"$OUTPUT_UUID"` with the same `kwriteconfig6` command. If the key was
previously absent, remove the added key:

```bash
kwriteconfig6 \
  --file kcminputrc \
  --group Libinput \
  --group "$FIRST_NUMERIC_GROUP" \
  --group "$SECOND_NUMERIC_GROUP" \
  --group "$TOUCHSCREEN_NAME" \
  --key OutputUuid \
  --delete \
  --notify
```

Log out and back in only if the restored mapping does not apply immediately.
