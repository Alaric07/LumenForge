# Changelog

This changelog covers LumenForge development beginning with the fork from
OpenLinkHub. Earlier upstream history remains available through the Git history
and the OpenLinkHub repository.

## Unreleased

### Fixed

- Corrected OpenRGB SDK protocol negotiation and controller parsing so imported
  devices use the zones and LED counts reported by the negotiated protocol.
- Removed obsolete hard-coded ASUS motherboard import compatibility.

### Upgrade Notes

- Existing OpenRGB imports preserve their saved layouts during upgrades. Users
  with an ASUS motherboard imported by 0.2.0-alpha must remove its complete
  entry from `database/openrgbimport-zones.json` and import it again to receive
  the corrected onboard and addressable-header zones. Using **Remove** in the
  web interface is not sufficient because it preserves the saved configuration.

## 0.2.0-alpha - 2026-07-29

### Added

- Added OpenRGB controller discovery and live import, removal, refresh, and
  reimport through Settings and the local management API without restarting
  LumenForge.
- Added stable OpenRGB controller identities and conservative, editable fallback
  layouts for controllers reporting zero LED metadata.
- Added the trusted External Source Registry for using explicitly approved local
  executables as temperature sources.
- Added dashboard backup creation and restore. Restore validates archives,
  bounds extraction, stages replacements with rollback protection, and loads
  restored state after a service restart.
- Added per-user systemd service installation for desktop use and system-tray
  support.
- Added coordinated graceful shutdown.
- Added and substantially expanded guides for configuration and filesystem
  layout, backup and restore, External Sources, OpenRGB import, cooling profiles
  and fan curves, troubleshooting, supported-device interpretation, and
  advanced hardware setup.

### Changed

- Restricted the dashboard, HTTP API, OpenRGB importer connections, and optional
  OpenRGB-compatible target listener to loopback addresses on the local machine.
- Protected the local HTTP service with Host validation and enforced
  same-origin and request-proof checks for browser mutations. Custom local API
  clients that modify state may need to send compatible local request headers.
- Made installed application files root-owned and immutable under
  `/opt/LumenForge`, while user and system configuration, profiles, uploads,
  generated state, and logs use separate XDG or `/var/lib/lumenforge` paths.
- Changed both installers to use transactional application staging and rollback,
  and to reject conflicting user-service and system-service installations.
- Preserved external runtime-owned configuration and mutable data across
  installer upgrades.
- Used the same mode-specific installer for installation and upgrade, removing
  the obsolete upgrade wrapper, remote installer, and inherited Docker
  packaging.
- Kept maintainer planning documents available in the repository while excluding
  them from generated Git archives and installed application content, and
  removed obsolete installed copies during upgrades.
- Gave fresh configuration files a stable, grouped field order.
- Preserved OpenRGB layouts and RGB/profile state across restart, removal, and
  reimport.
- Clarified that the LED-count warning means lighting updates are ignored rather
  than predicting an OpenRGB crash.
- Set ordinary development builds to report `0.2.0-alpha-dev` and allowed
  authorized release builds to embed an explicit version such as
  `0.2.0-alpha`.

### Fixed

- Fixed OpenRGB identity and lifecycle handling for incomplete metadata and
  mode-like locations such as `Direct`, `Dir`, and `Off`, preventing invalid or
  duplicate controllers while preserving disabled imports’ layouts, profiles,
  and RGB state for later reimport.
- Corrected fresh RGB animation defaults and calibrated Flame, Cyberpunk Glitch,
  Storm, and software Rain timing.
- Made cluster and individual animation speed controls consistently run from
  Slow to Fast, left to right.
- Hid irrelevant speed controls for static and temperature-based RGB profiles.
- Fixed the user-service tray action that opens the configured dashboard in the
  default browser.
- Restored AMD SMI GPU reporting for current `gpu_data` responses.
- Restored K65 Plus Wireless control-dial press actions and included configured
  labels in temperature-probe selectors.
- Corrected the K60 RGB PRO G-key lighting packet mapping and Link Adapter
  success feedback.
- Moved supported Corsair memory metadata out of hard-coded device logic while
  retaining safe built-in defaults.
- Updated the image-processing dependency to include current WebP and font
  parsing fixes.

## 0.1.0-alpha

- Initial LumenForge alpha release.
- Added OpenRGB device import.
- Added RGB Cluster workflows and physical device ordering.
- Added built-in themes.
- Added optional system tray integration.
