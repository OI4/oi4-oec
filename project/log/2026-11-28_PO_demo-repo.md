# 2026-11-28 – Demo repository `oi4-oec-demo` replaced by the OEC 2.0 demos

PO order (outside the cycles): "Ich möchte einen radikalen Move.
oi4-oec-demo ist nun total veraltet, es ist aber das demo repo. Daher
möchte ich, dass du alles von oi4-oec-demo löschst und alles was wir unter
oec für die Demos haben dort hin kopierst. Ich möchte danach also nur noch
die neuen Demos dort haben". STATUS and the C-112 log are not changed here.

## Situation

- `oi4-oec-demo/` in the workspace root: a fresh clone of `OI4/oi4-oec-demo`
  (**public**, branch `development`, head `d686c84`), the OEC 1.x demo
  (TypeScript/lerna, namespace `Oi4`, 159 files). It was untracked in the
  workspace; now ignored like the other separate repositories.
- `OI4/oec` (release repository, ADR-0034) does not exist yet and
  `OI4/oi4-oec` is private: the copy cannot build against a published SDK.
- `oec/demos` is used by other parts of `oec/` (registry image, tool image,
  `ci/sbom.sh`: `internal/sbom`, `images/licenses`) – it must stay; the
  order is a copy.

## Decision (agent, G-3.4 b; ADR-0012 amendment 2026-11-28)

Unmodified copy of `oec/demos` as `demos/` (the repository is laid out like
the OEC source tree with only its demos), no path rewrites: rewriting
`../sdk`, `../conformance`, `../spec`, `../ci`, the Docker context and the
harness root would fork the demos from their gate tests (`images_test.go`,
`demotest`). The README of the repository says that `oec/demos` is the
source and that the demos build and test in the OEC source tree
(`oec/demos/check.sh`). Alternatives not taken: rewrite to the release
modules of `OI4/oec` (not published, unverifiable now); vendor the SDK and
D7/D8 into the public repository (a publication, Q-011, and not "only the
demos").

## Done

- `oi4-oec-demo`: every tracked file removed, `demos/` = the 115 tracked
  files of `oec/demos` at workspace `71d8864`; new `README.md`, `LICENSE`
  (`oec/LICENSE`), `.gitignore`. One local commit `8c616d0` on
  `development`; **not pushed** (G-3.8, public repository).
- Workspace: `.gitignore` `/oi4-oec-demo/`; ADR-0012 amendment.

## Verification

- `git ls-files -s` of `oec/demos` and of `oi4-oec-demo/demos`: identical
  mode and blob for all 115 files (`diff` empty).
- `oi4-oec-demo` tree after the commit: `.gitignore`, `LICENSE`,
  `README.md`, `demos/` only; working tree clean.
- No private key or token pattern in the repository (`git grep`).
- `oec/` unchanged – no gate run needed.

## Re-sync after a change of `oec/demos`

From the workspace root (removes files deleted in `oec/demos`, keeps modes):

```sh
git -C oi4-oec-demo rm -r -q demos && rm -rf oi4-oec-demo/demos
git ls-files -z oec/demos | while IFS= read -r -d '' f; do
  d="oi4-oec-demo/demos/${f#oec/demos/}"; mkdir -p "$(dirname "$d")"; cp -p "$f" "$d"; done
git -C oi4-oec-demo add -A
```

## Open for the PO

- Push of `oi4-oec-demo` (`development`, and whether `main` follows) – a
  public publication (G-3.8, Q-011).
- Whether the old 1.x branches/tags of `OI4/oi4-oec-demo` stay.
