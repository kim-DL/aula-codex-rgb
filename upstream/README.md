# Vendored driver source

Origin: https://github.com/parsiya/f108-pro

Base revision: `f7782b02e254490d9c4ae063a8c27cae38be226f`.

This is a flattened source snapshot with local modifications for retained Windows HID realtime frames, bounded apply retries, preparation/timing metadata and volatile RTC synchronization. The original MIT license is retained in `LICENSE`.

It is intentionally not a nested Git repository or submodule. `app/go.mod` uses `replace github.com/parsiya/f108-pro => ../upstream` so the exact modified source builds with the app.

The legacy upstream CLI exposes low-level configuration capabilities beyond the resident application's volatile path. Hardware test files require an attached keyboard and must not run alongside the resident owner. Use the root `Build.ps1` for selected mock protocol tests and app builds.
