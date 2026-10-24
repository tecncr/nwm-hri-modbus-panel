# Dashboard

The web dashboard is compiled into the binary and served at the root (or
`server.base_path`). It is a client of the [REST API](api.md) — anything it
does can be scripted. Every option and button carries a tooltip explaining
what it does; hover or focus the **ⓘ** icons.

## Signing in

Enter the master API key. By default it is kept only for the browser tab
(session storage); tick **Remember on this device** to keep it in local
storage until you sign out. A `401` from the server signs you out
automatically.

## Languages

The dashboard is available in English and Spanish (Peru). The language is
chosen, in order, from:

1. the viewer's explicit choice in the language menu (the **文A** icon in the
   header or on the sign-in screen), stored in that browser;
2. the browser's preferred languages (`navigator.languages`, the same list it
   sends as `Accept-Language`), taking the first supported one; a regional
   variant such as `es-MX` selects Spanish;
3. `server.ui_language` (`NWM_PANEL_SERVER_UI_LANGUAGE`, `--ui-language`),
   which the server writes into `index.html` — the fallback for browsers that
   prefer none of the supported languages or send no preference.

Choosing *Automatic* in the menu forgets the explicit choice. Numbers and
dates follow the browser's regional variant of the chosen language
(`es-PE` or `en-US` when the browser has none). Schedule descriptions and build
placeholders reported by the server are translated by the dashboard; other
text produced by the server, such as error messages and reading warnings,
stays in English.

## Pages

| Page | Audience | What it shows |
|---|---|---|
| **Overview** | Everyone | Meters online, net consumption, recorder and database status; consumption by meter and over time for 1 h / 24 h / 7 d / 30 d or a custom range; the list of meters with their last readings. *Read all now* polls every meter live. |
| **Meter** (click a meter) | Everyone | Live reading (net, forward, reverse, resolution), warnings, consumption totals for the range, consumption per bucket and average flow charts, CSV download. |
| **Devices** | Administrators | Buses with traffic statistics; add, edit, delete, scan for encoders, synchronise all clocks, broadcast an address. Encoders with status; add, edit, delete. |
| **Registers** (per device) | Technicians | *Configuration*: slave address, clock, pulse equivalents, base readings, device type, reverse-flow threshold, diagnostics. *Register map*: every register, raw and decoded. *Raw access*: arbitrary reads and writes (functions 0x03/0x06/0x10). |
| **Measurements** | Analysts | The recorded database, filtered by meters and time, paginated, with CSV download of the whole selection and column choice. |
| **Exports** | Administrators | Export schedule, last run, generated files (download, delete), *Export now*. |
| **System** | Administrators | Version, uptime, inventory, database, recorder results per device, *Record now*, limits, and the effective configuration. |

Consequential actions (re-addressing, presetting counters, raw writes,
deletions, broadcasts) ask for confirmation.

## Design

The interface uses the stock shadcn/ui components with the neutral theme, fully
grayscale (every shade of gray, no hue), in light and dark variants, and the
Inter typeface (bundled, no external requests). The layout is responsive: the
navigation collapses into a menu on small screens and tables scroll
horizontally.

Only chart marks use color. Series colors follow a fixed categorical order
checked for color-vision deficiency against both the white and black
surfaces; a meter keeps its color regardless of ranking, multi-series charts
always have a legend, and every value is also available in tooltips and in
the Measurements table.

## Development

The dashboard lives in `web/`: React 19, TypeScript, Vite, Tailwind CSS 4 and
shadcn/ui (Radix), with TanStack Query for data fetching and Recharts for
charts.

```sh
# terminal 1: API server from source
export NWM_PANEL_API_KEY=dev
make dev

# terminal 2: dashboard with hot reload, proxying /api to 127.0.0.1:8080
cd web
pnpm install
pnpm dev
```

Set `NWM_PANEL_DEV_API` to proxy to another server. `pnpm build` writes
`web/dist`, which `go build` embeds (`make build` does both). `make lint` runs
`go vet`, the TypeScript compiler and oxlint.

Source layout:

| Path | Content |
|---|---|
| `src/lib/api.ts` | Fetch wrapper: relative URLs, Bearer auth, error envelope, downloads |
| `src/lib/types.ts` | API types |
| `src/i18n/` | i18next setup and language detection; `en.ts` holds the source strings, `es.ts` must provide the same keys (checked by TypeScript) |
| `src/pages/` | One file per page |
| `src/components/` | Shared pieces: charts, forms, dialogs, tooltips |
| `src/components/ui/` | stock shadcn/ui components |
| `src/index.css` | Theme tokens |
