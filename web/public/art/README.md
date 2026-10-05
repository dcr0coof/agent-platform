# Local artwork

All assets are bundled with the workspace. No remote image, font, animation or CDN requests occur at runtime.

- `travel-atlas.svg`: original project illustration, 2026-10-05. Decorative imaginary landscape, not a map or route result. Covered by the repository license.
- `weather.svg`: unchanged Meteocons `@meteocons/svg-static@0.1.0/fill/partly-cloudy-day.svg`, by Bas Milius, MIT. Source: https://cdn.jsdelivr.net/npm/@meteocons/svg-static@0.1.0/fill/partly-cloudy-day.svg ; upstream https://github.com/basmilius/meteocons . License: `METEOCONS-LICENSE`. Decorative only; does not indicate the destination's current weather.
- `compass.svg`: Lucide compass, downloaded 2026-10-05 from https://raw.githubusercontent.com/lucide-icons/lucide/main/icons/compass.svg . Source snapshot is vendored here; retain `LUCIDE-LICENSE` (ISC / Feather-derived MIT notice). Unchanged SVG, tinted with CSS for the brand mark.

The layout uses local system fonts, original CSS, static SVGs and short hover transitions. No WebGL, animation runtime or additional npm dependency. Reduced-motion preferences disable transitions. No third-party paid template or Hermes branding is included.
