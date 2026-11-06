# 2026-11-28 – Registry repository `oi4-oec-registry` replaced by the OEC 2.0 registry

PO order (outside the cycles, after the demo repository,
log 2026-11-28_PO_demo-repo): "Mach nun das Gleiche bitte für
oi4-oec-registry". STATUS and the C-112 log are not changed here.

## Situation

- `oi4-oec-registry/` in the workspace root: a clone of
  `OI4/oi4-oec-registry` (**public**, branch `development`, head `fb77c3e`,
  further remote branches `main`, `publish`, `split_ui`, …), the OEC 1.x
  registry (TypeScript/lerna, namespace `Oi4`, 90 files). Untracked in the
  workspace; now ignored.
- `oec/registry` (D5) reaches its siblings by relative paths: Cargo path
  dependencies `../sdk/rust/oec`, `../sdk/rust/oec-core`, the schemas
  `../schema/v2` in its API tests, D7/D8 from `../conformance` and the test
  broker of `../sdk/rust` in `check.sh`, the image build context `oec/`.
  `oec/demos/internal/sbom` reads `registry/ui/package-lock.json`.

## Decision (agent, G-3.4 b; ADR-0012 amendment 2026-11-28, registry)

The same as for the demos: unmodified copy of `oec/registry` as `registry/`
(laid out like the OEC source tree with only the registry), no path
rewrites; `oec/registry` stays the source. The `.gitignore` of the
repository ignores `dist/` but keeps the committed, embedded
`registry/ui/dist/` (as the workspace, ADR-0029 §9).

## Done

- `oi4-oec-registry`: every tracked file removed, `registry/` = the 62
  tracked files of `oec/registry` at workspace `0daa066`; new `README.md`,
  `LICENSE` (`oec/LICENSE`), `.gitignore`. One local commit `4fff53f` on
  `development`; **not pushed** (G-3.8, public repository).
- Workspace: `.gitignore` `/oi4-oec-registry/`; ADR-0012 amendment.

## Verification

- `git ls-files -s` of `oec/registry` and of `oi4-oec-registry/registry`:
  identical mode and blob for all 62 files (`diff` empty), incl. the four
  files of `ui/dist/`.
- Tree after the commit: `.gitignore`, `LICENSE`, `README.md`, `registry/`
  only; working tree clean.
- `git grep` for keys/tokens: only the PEM marker constants and the
  placeholder test data (`AAAA`) of `src/tls.rs`, no key.
- Re-sync below run on a temporary clone: no change (also with the
  `dist/` rule of the `.gitignore`).
- `oec/` unchanged – no gate run needed.

## Re-sync after a change of `oec/registry`

From the workspace root:

```sh
git -C oi4-oec-registry rm -r -q registry && rm -rf oi4-oec-registry/registry
git ls-files -z oec/registry | while IFS= read -r -d '' f; do
  d="oi4-oec-registry/registry/${f#oec/registry/}"; mkdir -p "$(dirname "$d")"; cp -p "$f" "$d"; done
git -C oi4-oec-registry add -A
```

## Open for the PO

- Push of `oi4-oec-registry` (`development`, and whether `main` follows) –
  a public publication (G-3.8, Q-011).
- Whether the old 1.x branches (`publish`, `split_ui`, `devcontainer`, …)
  and tags of `OI4/oi4-oec-registry` stay.

## Pushed (PO: "Du kannst alles pushen")

`oi4-oec-registry` `development` pushed to `OI4/oi4-oec-registry`. Fast-forward push `fb77c3e..4fff53f`. Secret scan before the push: clean (only the PEM markers of `src/tls.rs`). `main` and the
old 1.x branches and tags are left as they are (not part of the order;
`development` is the default branch). The workspace `main` was pushed
together (`71d8864..bb4fc24`).
