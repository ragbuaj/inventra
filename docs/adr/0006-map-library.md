# ADR-0006 — Map library: keep Leaflet + OSM (vs MapLibre / Google Maps)

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Backlog #2 |

## Context and problem statement

The office-location map ("Peta Lokasi", `OfficeMap.client.vue`) plots bank office locations as pins on
a slippy map with filter + detail. It is implemented with **Leaflet 1.9.4 + OpenStreetMap raster
tiles**, custom `divIcon` pins colored per office type, with `fitBounds`/`flyTo`. The backlog asked
whether to switch to **MapLibre GL** for a "built-in" map.

## Decision drivers

- Use case is **markers on a basemap** (offices), with click-to-select and a detail panel.
- Prefer **no vendor lock-in / no API key**; simple, free tiles.
- Bundle weight and complexity vs. actual benefit.
- Best-practice principle: industry-standard, not novelty for its own sake.
- Possible **scale**: a large bank (BTN) may have thousands of branches.

## Considered options

1. **Keep Leaflet + OSM raster** (current). ~40 KB, no key, simple API, already working; the map screen
   was a deliberate product decision over the mockup's illustrative SVG.
2. **MapLibre GL.** Vector tiles, GPU rendering, 3D/rotation, smooth zoom — but needs a **vector tile
   source** (MapTiler/OpenFreeMap/self-hosted) i.e. a new dependency/provider, heavier bundle (~200 KB+),
   and a component rewrite. Its strengths (vector styling, 3D, GPU point layers) don't materially help
   "pins on a map."
3. **Google Maps / Mapbox.** Paid, API-key, vendor lock-in. Rejected on principle (cost + lock-in).

## Decision outcome

**Chosen: Option 1 — keep Leaflet + OSM.** MapLibre adds a tile-source dependency and bundle weight
without improving the current use case; Google/Mapbox add cost and lock-in. Leaflet is the lighter,
vendor-neutral, industry-standard choice for marker maps and is already implemented and working.

## Consequences

- 👍 No API key, no vendor lock-in, small bundle, simple maintenance; no migration churn.
- 👎 Raster tiles (no vector styling / 3D) — not needed here.
- 👎 Rendering **thousands** of individual markers is slow in Leaflet → see "Revisit if".

## Revisit if

- Office/marker count grows large (≈1–2k+) and the map lags → first add **clustering**
  (`Leaflet.markercluster` or `supercluster`) or canvas rendering — still cheaper than a library swap.
- A requirement appears for **vector styling, rotation/pitch/3D, or offline/self-hosted vector tiles** —
  then reconsider **MapLibre GL** and supersede this ADR.
