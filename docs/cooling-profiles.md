# Cooling Profiles and Fan Curves

## What a cooling profile does

A cooling profile connects three parts of cooling control:

1. A **Sensor** supplies the measured temperature.
2. The profile converts that temperature into a requested **Fan Speed (%)** or
   **Pump Speed (%)**.
3. A supported fan, pump, controller, or motherboard channel is assigned to the
   profile.

The temperature-to-speed relationship is the fan curve. A **Static (no
temperature ranges)** profile instead requests one speed across its range.
Creating either kind of profile does not change hardware by itself. The profile
must also be assigned on a supported device or channel.

Profiles can be reused by multiple compatible channels. Hardware support and
minimum speeds vary, so a requested value is not a promise that every device
can produce that exact speed.

## Before changing cooling settings

- Keep a known-working BIOS, device-default, or current LumenForge profile
  available so you can restore it.
- Start while the system is idle, not under a heavy CPU or GPU load.
- Change and verify one device or channel at a time.
- Begin with a conservative curve instead of aggressively low speeds.
- Watch both temperatures and actual RPM while testing.
- Do not treat a pump exactly like a case fan unless its hardware documentation
  says that its speed range and stop behavior are safe.
- Leave **Zero RPM** off until you have verified the hardware and its thermal
  response.

There is no universal safe fan curve or percentage for every case, cooler,
pump, sensor location, and workload.

## Create a cooling profile

1. Open the local dashboard and select **Cooling Profiles** in the sidebar.
2. Select **New Cooling Profile**.
3. Enter a **Profile name**. It must contain at least three letters or numbers;
   spaces and punctuation are not accepted.
4. Select a **Sensor**. Additional selectors appear when LumenForge needs a
   particular storage device, probe, HWMON sensor, External Source, or GPU.
5. Enable **Static (no temperature ranges)** only when you want one requested
   speed rather than a temperature-based curve.
6. Leave **Zero RPM** disabled unless the intended controller and channel are
   known to support it safely.
7. Select **Create**.

Successful creation returns to **Cooling Profiles** and adds a card named after
the profile. LumenForge starts a new temperature-based profile from a built-in
curve appropriate to the selected sensor; it starts a static profile with one
speed range. Select the new card to review and change those initial values
before assigning it.

With the default graph editor, the profile shows separate pump and fan graphs.
Click empty graph space to add a point, drag a point to move it, and right-click
a point to remove it. Select **Save** below each graph you change. Each graph
has its own save action.

If the older table editor is enabled in `config.json`, select the profile, edit
**Fan Speed (%)** and **Pump Speed (%)**, and select **Update**. The
`graphProfiles` setting chooses the editor and requires a service restart; it
does not change where profiles are assigned.

## Assign the profile

Assignment happens on the page for the supported device, not on **Cooling
Profiles**:

1. Open the device from the dashboard.
2. Find the fan, pump, or other cooling channel.
3. Use that channel's **Profile** selector to choose the new profile. Some
   controllers also provide one selector for a chosen group of channels.
4. Confirm the success message and verify that the selected profile is shown.

The selection is applied and saved immediately; there is no separate device
save button and no service restart is normally required. A profile may be
assigned to several compatible channels. LumenForge rejects known mismatches,
such as a liquid-temperature profile on hardware without the required
temperature/pump path, but that validation is device-specific.

## Test safely

1. Start the system at idle.
2. Assign the profile to one channel.
3. Confirm that the expected fan or pump responds.
4. Confirm that the displayed RPM changes appropriately.
5. Apply a moderate workload only after idle behavior is correct.
6. Monitor the selected temperature source and cooling response.
7. Stop and restore the previous or **Normal** profile if temperature or RPM
   behaves unexpectedly.

This documentation pass did not physically test cooling hardware.

## Temperature sources

The current **Sensor** menu exposes these sources:

- **CPU**, **GPU**, and **CPU + GPU** use system temperature readings.
- **Multi GPU** selects one detected GPU; **Multi GPUs** uses the configured
  NVIDIA GPU index list and requires at least two configured entries.
- **Storage Temperature** selects a detected storage device.
- **External HWMON** selects another Linux HWMON temperature input.
- **Liquid Temperature (AIO)**, **Temperature Probe**, **Global Temperature
  Probe**, and **PSU** depend on temperature data from compatible LumenForge
  devices and can be rejected when the assigned hardware does not match.
- **External Source** selects a trusted local temperature command from the
  [External Source Registry](external-sources.md).

Only selectors backed by currently detected data are populated. A source that
disappears or cannot be parsed generally produces no current reading. An
External Source also produces no reading when its registry entry is missing or
invalid, its executable fails, its output is invalid, or it exceeds the
two-second execution timeout; the error is logged with its source ID.

Several native cooling implementations substitute `50 °C` when a temperature
read returns zero, but the final curve response and hardware command remain
device-specific. Do not treat that as a universal safe fallback or a guaranteed
speed. If the selected value is missing, zero, stale, or unexpected, treat the
profile as unavailable and restore a known-working profile.

## Zero RPM

**Zero RPM** allows a fan curve to request speeds below the normal software
minimum instead of having fan requests below 20% raised to 20%. The dashboard's
information panel currently identifies iCUE LINK System Hub, Commander Core XT,
Commander Core, and Commander Duo as available device families.

This option does not guarantee that every fan, controller, or channel can stop,
and pump minimum handling remains separate. Verify that the fan restarts after
the temperature rises. Do not assume Zero RPM is safe for a pump.

## Editing, unassigning, and deleting profiles

Select a custom profile card on **Cooling Profiles** to edit its fan and pump
values, then use the graph's **Save** buttons or the table editor's **Update**
button. The dashboard does not currently edit a saved profile's name, Sensor,
Static setting, or Zero RPM setting. To change those choices, create a
replacement profile and reassign the affected channels.

There is no separate unassigned state. On the device page, select the previous
profile or **Normal** for each channel you want to return to default behavior.
That selection is saved immediately.

Deleting a custom profile invokes LumenForge's native-device reset handling:
assigned channels covered by that handling are changed to **Normal**, and the
updated device profile is saved. Because device implementations differ, confirm
every affected channel after deletion rather than assuming its physical
response.

## Troubleshooting

See [Troubleshooting](troubleshooting.md#cooling-profile-problems) for a short
checklist. Common causes include a saved profile that was never assigned, the
wrong or unavailable Sensor, an unsupported channel, and `manual: true` in
`config.json`. Manual mode keeps telemetry running but disables automatic
cooling-curve adjustment and prevents profile assignment until it is disabled
and the matching service is restarted.

## Technical notes

Custom profiles are JSON state under the active service mode's mutable data
root in `database/temperatures/`. Channel assignments are stored with each
device's profile state. For a user service, the mutable root is
`$XDG_DATA_HOME/lumenforge` or `~/.local/share/lumenforge`; for a system service
it is `/var/lib/lumenforge`. Manage profiles through the dashboard and do not
edit the installed application tree under `/opt/LumenForge`.
