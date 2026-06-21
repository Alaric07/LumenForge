# OpenRGB Device Import

LumenForge can use a running OpenRGB SDK Server as a bridge to devices supported by OpenRGB. In this primary LumenForge workflow, OpenRGB provides device access and LumenForge acts as an SDK client/importer:

```text
OpenRGB-supported device -> OpenRGB SDK Server -> LumenForge -> Dashboard / RGB Cluster
```

Imported devices can appear alongside LumenForge's native devices and participate in dashboard and RGB Cluster workflows where the available OpenRGB metadata and LumenForge's importer support it.

OpenRGB support alone does not guarantee complete LumenForge support. Device names, zones, LED counts, effects, and control behavior vary by vendor and by the metadata exposed through the OpenRGB SDK protocol.

## Setup

1. Open the official OpenRGB application, or start a headless OpenRGB instance with its SDK Server enabled.
2. In the OpenRGB GUI, open the **SDK Server** tab and start the server.
3. Confirm the SDK Server is listening on `127.0.0.1:6742`, or note the custom port you selected.
4. Set `openRGBPort` in LumenForge's `config.json` to the same port. New LumenForge configs default to `6742`.
5. Start or restart LumenForge.
6. Open the LumenForge dashboard and verify the imported devices and their zone layouts.

LumenForge currently connects to OpenRGB on `127.0.0.1`. The SDK Server must therefore be reachable on the same machine and port configured by `openRGBPort`.

## Import Configuration

LumenForge stores the last known OpenRGB zone layout in:

```text
database/openrgbimport-zones.json
```

This is generated runtime state and is not shipped with personal device data. When the file is missing, LumenForge creates an empty store and populates it as devices are discovered.

For each imported device, the saved configuration is the source of truth:

- First discovery creates a best-effort starting layout.
- Saved zone names and LED counts persist across restarts.
- Automatic discovery does not overwrite an existing saved layout.
- Users can correct zone names and LED counts when OpenRGB metadata is incomplete.

To reset all imported layouts, stop LumenForge and remove `database/openrgbimport-zones.json`. LumenForge will regenerate it on the next import. Back up the file first if you may want the current layouts later.

## Device and Zone Limitations

LumenForge favors stable saved layouts over aggressive protocol parsing because OpenRGB controller payloads vary significantly between devices and vendors.

- Known device families may receive conservative starting layouts.
- Unknown devices may begin with a minimal one-zone layout.
- LED counts are best-effort defaults and should be checked against OpenRGB.
- Some devices may import only partially or may not expose enough metadata for useful control.
- Imported effects and controls depend on both the OpenRGB device implementation and LumenForge's importer.

## LED Count Safety

Sending an invalid or excessive LED count can destabilize some OpenRGB device implementations or the SDK Server. When a zone count is increased, LumenForge applies the candidate layout and checks that OpenRGB remains reachable before saving it.

Use the OpenRGB UI as the reference for zone structure and LED counts. Make small changes and confirm device behavior before increasing counts further.

## Two OpenRGB Directions

LumenForge contains two distinct OpenRGB integrations:

1. **Import into LumenForge (primary):** LumenForge connects as an SDK client to an external or headless OpenRGB SDK Server and imports OpenRGB-backed devices into the LumenForge UI.
2. **Expose LumenForge devices to OpenRGB (inherited/secondary):** inherited OpenLinkHub functionality can run a legacy OpenRGB-compatible target listener so an OpenRGB client can control supported native devices.

The second direction does not import OpenRGB-backed devices into LumenForge and is not the primary workflow documented here. Its older screenshots and instructions are retained separately in [`openrgb/README.md`](../openrgb/README.md) for inherited compatibility and are explicitly labeled as such.
