# Third-party notices

- `upstream/` is a source snapshot of [parsiya/f108-pro](https://github.com/parsiya/f108-pro), based on commit `f7782b02e254490d9c4ae063a8c27cae38be226f`, with local Windows realtime RGB, clock, transport and test changes. Its MIT notice is retained in `upstream/LICENSE` and `app/LICENSE-parsiya.txt`.
- Realtime protocol behavior and the 104-address ordering were cross-checked against the MIT-licensed [Punkster81/AULA-F108-Driver](https://github.com/Punkster81/AULA-F108-Driver). This app uses protocol/order data, not that desktop application. The reference license is included in `licenses/`.
- Runtime dependencies are pinned by `app/go.mod` and `app/go.sum`, including go-winio, systray, x/sys and transitive modules. Their licenses remain available with their respective source distributions.
- This repository does not select a new blanket license for the local application. Existing third-party licenses and notices remain applicable to their components.
