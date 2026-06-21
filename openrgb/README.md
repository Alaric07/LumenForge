# Legacy OpenRGB Target Server (Inherited)

> [!IMPORTANT]
> This document describes the inherited, secondary direction where LumenForge exposes supported native devices to an OpenRGB client. It does **not** describe importing OpenRGB-backed devices into LumenForge. For the primary LumenForge workflow, see [OpenRGB Device Import](../docs/openrgb-import.md).

This inherited OpenLinkHub-era functionality allows LumenForge to expose supported native devices through an OpenRGB-compatible target server.

This effectively resolves any device communication issue that occurs when two programs attempt to communicate with the same device. 

With this implementation, you can utilize OpenRGB for your RGB effects and use LumenForge for temperature monitoring, fan control, pump control, LCD control, and everything else the program offers you. 

## How to configure
### Step 1
```json
{
  "enableOpenRGBTargetServer": true,
  "openRGBPort": 6743
}
```

- `enableOpenRGBTargetServer` This will enable TCP listener
- `openRGBPort` TCP port to listen on. This is OpenRGB native server port + 1

### Step 2
```bash
systemctl stop LumenForge
```

### Step 3
Disable device in the OpenRGB application, so it's not processed there and click Apply button. After that you can either Rescan Devices or restart OpenRGB application. 
For each device you want integration, you'll have to disable it in OpenRGB. 

![Device disabled in OpenRGB for inherited target-server control](../static/img/openrgb-device.png)

### Step 4
```bash
systemctl start LumenForge
```

### Step 5
- Toggle OpenRGB Integration

![Inherited OpenRGB integration toggle in LumenForge](../static/img/openrgb.png)

### Step 6
In OpenRGB, click on Client tab connect to 6743 port. 

![OpenRGB client connected to the inherited LumenForge target server](../static/img/openrgb-client.png)

## Supported devices
| Device                 |
|------------------------|
| iCUE LINK System Hub   | 
| iCUE COMMANDER Core    |
| iCUE COMMANDER Core XT |
| iCUE COMMANDER DUO     |
| ELITE AIOs             |
| PLATINUM AIOs          |
| HYDRO AIOs             |
| Memory                 |
| MM700                  |
| MM800                  |

Supported devices may change as inherited compatibility and LumenForge support evolve.
