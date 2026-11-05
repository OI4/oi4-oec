# ADR-0034 – Publishing a release (P9)

- Status: **accepted (agent, G-3.4 b)** for the publishing step as built: `oec/ci/publish.sh`, the job `publish` of `release.yml`, the dry run in every release run, the preparation of the release repository, the order and the secret handling. **Decisions of the PO** (G-3.4 a, Q-011): the channels (C-109 D-3 (a)), the visibility of `OI4/oec` and the packages, the repository, the environment with its reviewers, the token and the first release tag itself (C-110 D-2).
- Date: 2026-11-26
- Deciders: agent (design, script, workflow); product owner (Thomas) – the publication, its prerequisites and the go
- Related: ADR-0033 (versioning, release build; amendment C-109 addendum 1 = C-109 D-1 … D-3 (a)), ADR-0032 §4 (CI policy) and amendment C-110, C-104 D-3 (a) (release repository `OI4/oec`, `oec/` as its root, module paths unchanged), C-108 D-3 (a) (weekly supply-chain run and Dependabot from the first release), Q-011 (publication, CRA), A-613, ROADMAP P9 increment 7

## Context

C-109 built `release.yml`: every deliverable from one commit into the workflow artifact `oec-release` (files, `SHA256SUMS`, `RELEASE-NOTES.md`); nothing is published. The PO then chose the channels (C-109 D-3 (a)): a **GitHub Release on `OI4/oec`** with the files as assets, the five images on **`ghcr.io/oi4/…`** (multi-arch), and the **Go modules through their tags** in `OI4/oec`. `OI4/oec` does not exist yet; its root will be `oec/` of the workspace (C-104 D-3 (a)).

Three things make the publication more than an upload:

1. **Go modules in subdirectories** are found by tags `<module directory>/v<version>` (ADR-0033 §5). Three of the twelve modules (`conformance`, `demos`, `schema/tools`) reach their siblings through `replace` directives to local paths; the go command ignores `replace` in a dependency, so `go install github.com/OI4/oec/conformance/cmd/oec-check@v0.1.0` would fail. The released `go.mod` files must require the sibling modules at the release version, and the `go.sum` lines of those versions only exist once the tags exist.
2. **The images** are OCI archives with a multi-arch index (C-107, C-109). A push must keep the index and its digest – the D8 reports of the release judged exactly those bytes.
3. **A publication cannot be taken back** (a Go module version is cached by the proxy forever, a release is seen at once): everything must be checked before the first write, and each release run should exercise that check long before the first real one.

## Decision

### 1. One script, a dry run by default (A-619)

`oec/ci/publish.sh <dir> [--publish]` takes the assembled release of `release.sh` and

1. **checks it:** `SHA256SUMS` matches and lists exactly the files of `<dir>`; the version (first line of `RELEASE-NOTES.md`) is a release or snapshot version; the commit and its time from the notes; the source archive is there;
2. **prepares the release repository** (§2) and checks it with an empty module cache;
3. **checks the images:** every `oec-<image>-<v>.oci.tar` has an image index with `linux/amd64` and `linux/arm64` (`skopeo inspect --raw`); their digests (`skopeo manifest-digest`) go into the release body;
4. **writes the release body:** `RELEASE-NOTES.md` plus "Where to get it" (files and `sha256sum -c`, the Go module tags with the `go install` command and the note that the Go SDK needs the `liboec_ffi` of the same version, the images with their digests);
5. **publishes** – only with `--publish` (§3).

Without `--publish` every command that writes to GitHub or GHCR is printed (`publish: dry run, would run: …`) through one function `remote`; `TestPublish` requires every line that pushes, copies to a registry or calls `gh release`/`gh api` to go through it. The job `release` of `release.yml` runs the dry run on **every** run (snapshot or tag) before the upload of `oec-release`, so the hosted runs of C-111 and every later snapshot exercise the whole preparation without a token.

### 2. The release repository commit (A-620)

- The tree is `oec/` of the source archive (`--strip-components=1`), in a fresh `git init` – the history of the workspace (internal logs, the guideline findings) does not go to `OI4/oec`; the commit message names the source commit.
- In each module with a `replace` to `github.com/OI4/oec/…`: `go mod edit -dropreplace=<path> -require=<path>@v<version>`; any other `replace`, or one left over, fails the run; each module path must be `github.com/OI4/oec/<directory>` (the tag prefix).
- **`go.sum` from the repository itself:** the prepared tree is committed and tagged; `go mod tidy` runs in the edited modules with an empty `GOMODCACHE`, `GOPRIVATE=github.com/OI4/oec` (no proxy, no checksum database for our modules) and git's `url.file://<work>.insteadOf https://github.com/OI4/oec` – the go command fetches the sibling modules from the local tags. Commit, retag, repeat until nothing changes (at most five rounds; one is enough with dependency depth 1). Modules of other authors come from the module cache of the runner, else from the proxy, as in the gate.
- The final commit has the **time of the source commit** and a fixed author/committer (`OEC release`, the address of `github-actions[bot]`): the same release gives the same tree and, without a parent, the same commit (C-110: the same commit id on macOS and in Ubuntu 24.04). With `--publish` its parent is the current `main` of `OI4/oec` (a fast-forward push; an empty repository gives a root commit).
- Tags: `v<version>` (the train) and `<module directory>/v<version>` for each of the twelve Go modules, lightweight, on the final commit.
- **Check before the publication, with a fresh cache:** in every module `go list -mod=readonly -deps -test ./...` and `go mod verify`; then `go install github.com/OI4/oec/conformance/cmd/oec-check@v<v>` and `…/oec-infra@v<v>` outside any module with the toolchain of `conformance/go.mod`; both must run `version`.

### 3. Publication only on the tag, behind an environment (A-621)

- Job **`publish`** of `release.yml`: `needs: release`, `if: github.ref_type == 'tag'`, **`environment: release`**, `ubuntu-24.04` (Skopeo and the GitHub CLI are preinstalled), 60 minutes; checkout without credentials, Go of `oec/ci/go.mod`, the artifact `oec-release` of the same run, one run step `oec/ci/publish.sh dist/release --publish` with `OEC_RELEASE_TOKEN: ${{ secrets.OEC_RELEASE_TOKEN }}` and `OEC_RELEASE_USER: ${{ vars.OEC_RELEASE_USER }}` in its `env`.
- The **environment `release`** is the PO's gate: with the PO as required reviewer, every publication waits for an approval in the GitHub UI after the release build is green; its secret exists only for jobs in it. The workflow stays `contents: read` – the job writes with the token, not with `GITHUB_TOKEN` (which cannot write another repository or the packages of the organisation).
- `publish.sh --publish` refuses anything but the build of the tag `oec-v<version>` (`GITHUB_REF_TYPE=tag`, `GITHUB_REF_NAME`) of a release version (no snapshot), and needs the token, the user, `gh` and `skopeo`.
- The model `oec/ci/gha` (ADR-0032 amendment C-110): job `if` and `environment`; an environment secret only in the `env` of a run step of a job in that environment; no environment in a workflow with a `pull_request` trigger; `if:` conditions are checked for untrusted contexts like any expression.

### 4. Order, idempotency, secrets (A-622)

- **Order:** (1) one atomic `git push` of `main` and the tags to `OI4/oec`; (2) the images, `skopeo copy --all --preserve-digests --retry-times 3 oci-archive:<file> docker://ghcr.io/oi4/oec-<image>:<v>` – the index and every platform with their digests; (3) `gh release create v<v> --verify-tag` with every file of the release as an asset. The GitHub Release comes last: it is what users see, and it appears only when the tags and images it lists exist.
- **Rerun:** a tag of `OI4/oec` that exists with the same tree is kept (nothing pushed for it), with another tree the run fails; an existing GitHub Release fails the run before anything is written. A rerun after a failed image copy therefore skips the repository part, copies the images again (same digests) and creates the release.
- **Secrets:** the token never appears in an argument, a URL or the log: git gets it as an `http.https://github.com/.extraheader` from `GIT_CONFIG_*` environment variables, skopeo from an authfile with mode 600 in the temporary directory (removed on exit), gh from `GH_TOKEN`; no `set -x` (`TestPublish`). git runs without the host's configuration (`GIT_CONFIG_NOSYSTEM`, `GIT_CONFIG_GLOBAL=/dev/null`).

### 5. What the PO provides (C-110 D-2)

The repository `OI4/oec` (visibility Q-011), the environment `release` in `OI4/oi4-oec` with required reviewers and the tag rule `oec-v*` (created before the first tag – GitHub creates a missing environment without protection), its secret `OEC_RELEASE_TOKEN` and variable `OEC_RELEASE_USER`, and the visibility of the GHCR packages after the first push (GHCR creates them private). GHCR accepts only classic personal access tokens (`write:packages`); with `repo` the same token writes `OI4/oec` – best of a machine account that has access to `OI4/oec` only. The steps are in `oec/ci/README.md` ("Activation").

## Consequences

- The first release is one tag plus one approval; the same path has run as a dry run on every release build before.
- `OI4/oec` gets one commit per release, without the workspace history; changes between releases are in `CHANGELOG.md`, the internal logs stay in `OI4/oi4-oec`.
- The released `go.mod` files differ from the workspace ones (no `replace`, sibling requirements at `v<v>`, more `go.sum` lines); the workspace keeps its `replace` directives for development.
- D7/D8 installed with `go install …@v<v>` report the module version (`report.ToolVersion`, an additive change, test first); a `go install` from the sources still says `dev`.
- Not checkable before the first release: the real push, the GHCR login with the PO's token, `gh release create`, and the environment approval; the dry run covers everything else (C-111 on the hosted runners).
- The weekly `supply-chain.yml` run and Dependabot on `OI4/oec` come with the first release (C-108 D-3 (a)), as do the C-110 work-group handover and the CRA review (Q-011) – outside this ADR.

## Alternatives considered

- **Push the workspace history (subtree split of `oec/`)** to `OI4/oec`: keeps the history, but the commits name internal cycles, review items and findings, and a split rewrites ids – rejected; one commit per release from the archive that `SHA256SUMS` covers.
- **Keep `replace` and publish only the binaries:** `go install …@v` and the Go SDK as a module would not work – rejected (C-109 D-3 (a) names the Go module tags as a channel).
- **`go.sum` from the module proxy after a first push:** needs a pushed tag before the check, and a broken `go.mod` would be public – rejected for the local repository with `insteadOf`.
- **`docker buildx imagetools` / `docker push`:** needs a daemon or a load of the archive (the classic Docker store cannot hold a multi-arch index, C-109) – rejected for skopeo, which copies the OCI layout as it is.
- **`GITHUB_TOKEN` with `packages: write`:** cannot push to another repository, and the workflow would need write permissions (ADR-0032 §4) – rejected.
- **A separate workflow on `workflow_run`:** the trigger is not in the model (ADR-0032 §4: it runs with the secrets of the default branch for untrusted input) – rejected for a job in `release.yml` behind an environment.
- **GitHub's own action for releases or attestation actions:** only `actions/*` are allowed, and `gh` is on the runner – not needed.
