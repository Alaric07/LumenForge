# Native Lighting Migration Matrix

## Purpose

This document is the package-level safety inventory for native RGB migration.

The migration must preserve support for every currently supported native device.
A package remains on the legacy RGB implementation until equivalent canonical
lighting behavior is implemented and validated. Matching architectural
signatures are audit aids only; they do not by themselves prove that packages
are safe to migrate together.

## Status definitions

- **Legacy** — current legacy RGB implementation remains authoritative.
- **Audit Required** — the package only matched weak lighting markers and must
  be inspected manually before deciding whether it is a migration target.
- **Audited** — current lighting behavior and hardware-specific capabilities
  have been documented.
- **Migration Ready** — canonical replacement behavior and validation
  requirements are defined.
- **Migrated** — the package/family uses canonical lighting and no longer
  participates in the corresponding legacy lighting path.
- **Validated** — automated parity has passed and hardware testing has been
  completed where hardware is available.
- **Deferred** — intentionally remains on legacy because safe parity has not yet
  been established.
- **Not a lighting target** — manual audit proved the package does not own a
  native lighting implementation requiring migration.

## Migration invariants

A native package must not lose existing supported behavior during migration.

Before a migrated package can be considered complete, its applicable behavior
must be accounted for, including:

- selected effect and renderer behavior;
- Brightness semantics;
- device-specific lighting modes;
- zone, per-key, per-channel, or per-LED behavior;
- RGB Cluster membership and output where supported;
- OpenRGB target-server integration where supported;
- scheduler/lights-out and Off behavior;
- non-lighting user-profile behavior;
- restart, reconnect, and profile-switch behavior;
- persistence and rollback behavior;
- automated regression coverage;
- real-hardware validation where hardware is available.

The legacy `/rgb` editor and remaining global RGB mutation infrastructure stay
available for unmigrated packages until every remaining consumer has parity.

---

# LumenForge Native Lighting Migration Inventory

Generated as a read-only architecture inventory. This does not declare migration parity or hardware support status.

## Summary

- Packages with any scanned lighting marker: **137**
- Strong legacy-lighting candidates: **131**
- Weak-marker-only packages requiring manual review: **6**

### Structural shapes

- zoned: **64**
- single-profile: **52**
- multi-channel: **9**
- weak-marker only: **6**
- multi-channel + per-LED: **4**
- other lighting: **2**

## Package matrix

| Package | Strong | Status | Shape | Legacy RGB | Brightness | Override | Zones | Per LED | Cluster | OpenRGB target | Special modes |
|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| `cc` | Y | Legacy | multi-channel | Y | Y | Y |  |  | Y | Y | led, liquid-temperature |
| `ccxt` | Y | Legacy | multi-channel | Y | Y | Y |  |  | Y | Y | probe-temperature |
| `cduo` | Y | Legacy | multi-channel | Y | Y | Y |  |  | Y | Y | probe-temperature |
| `clipperpromini60` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `cone` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led, liquid-temperature |
| `cpro` | Y | Legacy | multi-channel | Y | Y |  |  |  | Y | Y |  |
| `darkcorergbproW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproseW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproseWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbseW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `darkcorergbseWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `darkstarW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkstarWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `elite` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led, liquid-temperature |
| `glaivergb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `glaivergbpro` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `harpoonW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `harpoonWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `harpoonrgbpro` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `hs80maxW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `hs80rgb` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `hs80rgbW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `hs80rgbWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `hydro` | Y | Legacy | multi-channel | Y | Y |  |  |  |  |  |  |
| `ironclaw` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawSEW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawSEWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `k100` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k100airW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k100airWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k55` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k55core` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k55coretkl` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k55pro` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k55proXT` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `k57rgbW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k57rgbWU` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `k60rgbpro` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `k65plusW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k65plusWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k65pm` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k65rgb` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k65rgbRF` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k65rm` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k68rgb` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70core` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70coretkl` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70coretklW` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `k70coretklWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70lux` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70luxrgb` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70max` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70mk2` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70pmW` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k70pmWU` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k70pro` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70protkl` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70rgbRF` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70rgbtklcs` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k95` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k95platinum` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k95platinumXT` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `katarpro` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `katarproW` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  |  |
| `katarproxt` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `lncore` | Y | Legacy | multi-channel | Y | Y |  |  |  |  |  |  |
| `lnpro` | Y | Legacy | multi-channel | Y | Y |  |  |  |  |  |  |
| `lsh` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led, liquid-temperature, probe-temperature |
| `lt100` | Y | Legacy | multi-channel | Y | Y |  |  |  | Y | Y |  |
| `m55` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  |  |
| `m55W` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  |  |
| `m55rgbpro` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65prorgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbelite` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultra` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultraW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultraWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m75` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `m75AirW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `m75AirWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `m75W` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `m75WU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `makr75W` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `makr75WU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `memory` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led |
| `mm700` | Y | Legacy | single-profile | Y | Y |  |  |  | Y | Y | mousepad |
| `mm800` | Y | Legacy | single-profile | Y | Y |  |  |  | Y | Y | mousepad |
| `motherboard` |  | Audit Required | weak-marker only |  |  |  |  |  |  |  |  |
| `nautilusLcd` | Y | Legacy | other lighting | Y |  |  |  |  |  |  |  |
| `nexus` |  | Audit Required | weak-marker only |  |  |  |  |  |  |  |  |
| `nightsabreW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `nightsabreWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `nightswordrgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `platinum` | Y | Legacy | multi-channel | Y | Y |  |  |  |  | Y | liquid-temperature |
| `psudongle` |  | Audit Required | weak-marker only |  |  |  |  |  |  |  |  |
| `psuhid` |  | Audit Required | weak-marker only |  |  |  |  |  |  |  |  |
| `sabreprocs` | Y | Legacy | other lighting | Y |  |  |  |  |  |  |  |
| `sabrergbpro` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrergbproW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrergbproWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrev2proW` | Y | Legacy | zoned |  | Y |  | Y |  |  |  | mouse |
| `sabrev2proWU` | Y | Legacy | zoned |  | Y |  | Y |  |  |  | mouse |
| `scimitar` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarSEW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarSEWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarprorgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarrgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarrgbelite` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scufenvisionproV2W` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `scufenvisionproV2WU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `scufenvisionproW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `scufenvisionproWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `slipstream` |  | Audit Required | weak-marker only |  |  |  |  |  |  |  |  |
| `st100` | Y | Legacy | single-profile | Y | Y |  |  |  | Y | Y | stand |
| `strafergbmk2` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `vanguard96` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96W` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96WU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96pro` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard99airW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard99airWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `virtuosoSEW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosoSEWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosoW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosoWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosomaxW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosorgbXTW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosorgbXTWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `voidV2W` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `voideliteW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `xc7` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | liquid-temperature |
| `xeneonedge` |  | Audit Required | weak-marker only |  |  |  |  |  |  |  |  |

## Architectural signature groups

These groups are only an audit shortcut. Matching signatures do NOT mean packages are automatically safe to migrate together.

### zoned + cluster + openrgb-target + special:mouse

Count: **35**

`darkcorergbproW`, `darkcorergbproWU`, `darkcorergbproseW`, `darkcorergbproseWU`, `darkstarW`, `darkstarWU`, `glaivergb`, `glaivergbpro`, `harpoonW`, `harpoonWU`, `ironclaw`, `ironclawSEW`, `ironclawSEWU`, `ironclawW`, `ironclawWU`, `m55rgbpro`, `m65prorgb`, `m65rgbelite`, `m65rgbultra`, `m65rgbultraW`, `m65rgbultraWU`, `nightsabreW`, `nightsabreWU`, `nightswordrgb`, `sabrergbpro`, `sabrergbproW`, `sabrergbproWU`, `scimitar`, `scimitarSEW`, `scimitarSEWU`, `scimitarW`, `scimitarWU`, `scimitarprorgb`, `scimitarrgb`, `scimitarrgbelite`

### single-profile + cluster + special:keyboard

Count: **27**

`clipperpromini60`, `k100`, `k100airW`, `k100airWU`, `k57rgbW`, `k65plusW`, `k65plusWU`, `k65pm`, `k65rm`, `k70core`, `k70coretkl`, `k70coretklWU`, `k70max`, `k70pmW`, `k70pmWU`, `k70pro`, `k70protkl`, `k70rgbtklcs`, `k95platinum`, `makr75W`, `makr75WU`, `vanguard96`, `vanguard96W`, `vanguard96WU`, `vanguard96pro`, `vanguard99airW`, `vanguard99airWU`

### zoned

Count: **19**

`hs80maxW`, `hs80rgb`, `hs80rgbW`, `hs80rgbWU`, `m75AirW`, `m75AirWU`, `scufenvisionproV2W`, `scufenvisionproV2WU`, `scufenvisionproW`, `scufenvisionproWU`, `virtuosoSEW`, `virtuosoSEWU`, `virtuosoW`, `virtuosoWU`, `virtuosomaxW`, `virtuosorgbXTW`, `virtuosorgbXTWU`, `voidV2W`, `voideliteW`

### single-profile + special:keyboard

Count: **18**

`k55`, `k55core`, `k55coretkl`, `k55pro`, `k55proXT`, `k57rgbWU`, `k60rgbpro`, `k65rgb`, `k65rgbRF`, `k68rgb`, `k70coretklW`, `k70lux`, `k70luxrgb`, `k70mk2`, `k70rgbRF`, `k95`, `k95platinumXT`, `strafergbmk2`

### zoned + special:mouse

Count: **10**

`darkcorergbseW`, `darkcorergbseWU`, `harpoonrgbpro`, `katarpro`, `katarproxt`, `m75`, `m75W`, `m75WU`, `sabrev2proW`, `sabrev2proWU`

### weak-marker only

Count: **6**

`motherboard`, `nexus`, `psudongle`, `psuhid`, `slipstream`, `xeneonedge`

### multi-channel

Count: **3**

`hydro`, `lncore`, `lnpro`

### single-profile

Count: **3**

`katarproW`, `m55`, `m55W`

### multi-channel + cluster + openrgb-target

Count: **2**

`cpro`, `lt100`

### multi-channel + override + cluster + openrgb-target + special:probe-temperature

Count: **2**

`ccxt`, `cduo`

### multi-channel + per-LED + override + cluster + openrgb-target + special:led,liquid-temperature

Count: **2**

`cone`, `elite`

### other lighting

Count: **2**

`nautilusLcd`, `sabreprocs`

### single-profile + cluster + openrgb-target + special:mousepad

Count: **2**

`mm700`, `mm800`

### multi-channel + openrgb-target + special:liquid-temperature

Count: **1**

`platinum`

### multi-channel + override + cluster + openrgb-target + special:led,liquid-temperature

Count: **1**

`cc`

### multi-channel + per-LED + override + cluster + openrgb-target + special:led

Count: **1**

`memory`

### multi-channel + per-LED + override + cluster + openrgb-target + special:led,liquid-temperature,probe-temperature

Count: **1**

`lsh`

### single-profile + cluster + openrgb-target + special:stand

Count: **1**

`st100`

### single-profile + special:liquid-temperature

Count: **1**

`xc7`
