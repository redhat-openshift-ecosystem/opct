---
name: opct-release
description: Release a new OPCT version — release candidate (vX.Y.Z-rc.N) or GA (vX.Y.Z). Covers both repos (opct CLI, opct-plugins), the openshift/release CI image bump, the GitHub release page fixes, and recovery from failed tag builds. Use whenever cutting, re-cutting, or validating an OPCT release.
allowed-tools: Bash, Read, Edit, Write, Grep, Glob, WebFetch
---

# OPCT Release

End-to-end runbook for shipping an OPCT version. Written so an engineer who has
never cut an OPCT release can do it without tribal knowledge.

## Scope and vocabulary

Two repositories ship together:

| Repo | What it produces | Release branch |
|------|------------------|----------------|
| `redhat-openshift-ecosystem/opct` | CLI binaries + `quay.io/opct/opct` | `release-X.Y` (e.g. `release-0.7`) |
| `redhat-openshift-ecosystem/opct-plugins` | `quay.io/opct/plugin-openshift-tests`, `plugin-artifacts-collector`, `must-gather-monitoring` | `release-X.Y` (e.g. `release-0.7`) |

**Branches and tags are named differently — this trips people up:**

| | Format | Example | `v` prefix? |
|---|---|---|---|
| Release branch | `release-X.Y` | `release-0.7` | **no** |
| GA tag | `vX.Y.Z` | `v0.7.0` | **yes** |
| RC tag | `vX.Y.Z-rc.N` | `v0.7.0-rc.0` (N starts at 0) | **yes** |

Both repos follow this. Older docs saying the plugins branch is `release-vX.Y`
are wrong — there is no such branch in either repo.

`opct-plugins` was renamed from `provider-certification-plugins`; the old URL
redirects, but never use it in new docs or commands.

A third repo, `openshift/release`, holds the CI job definitions that consume the
published CLI image. Bumping it is a mandatory release step (Step 4).

## The one rule that governs everything

`pkg/types.go` in the CLI repo pins the plugin image tags. CI enforces it:

- On `main`, all four image constants are `:latest`. Never pin on main.
- On a **push to** a `release-*` branch, the `verify-images` job **fails if any
  of the four constants is `:latest`** (`.github/workflows/go.yaml`, "Block
  latest tags on release branches").
- `verify-images` also checks the pinned tags **actually exist on quay** — but
  only the three plugin constants (`PluginsImage`, `CollectorImage`,
  `MustGatherMonitoringImage`). `ControllerImage` is never existence-checked.

Consequence: **plugins must be tagged and their images published before the CLI
release PR can go green.** Always Step 2 then Step 3, never the reverse.

**Two gaps the job does not cover — check them by hand:**

1. The block-`latest` guard keys on `startsWith(github.ref, 'refs/heads/release-')`.
   On a pull request `github.ref` is `refs/pull/N/merge`, and on a tag build it is
   `refs/tags/...`, so **the guard does not run in either case**. A PR targeting a
   release branch can leave a constant at `:latest`, pass CI (the `:latest` tags
   do exist on quay), and get merged. Diff `pkg/types.go` yourself before approving
   a release PR.
2. `ControllerImage` is not existence-checked anywhere. It points at the CLI image
   this very release will build, so at pin time the tag legitimately does not exist
   yet — just confirm it matches the version you are about to tag.

## Decide the flow first: minor vs patch

**Minor bump** (`0.6.x` → `0.7.0`) where `release-X.Y` **does not exist yet** —
typically the first RC of that minor:
- Create it by pushing `main` to the new branch name. The branch is a copy of
  main's head, so **no merge PR is needed to seed it**.
- Branch protection has not blocked initial branch creation in practice
  (verified creating `release-0.7` in both repos).
- The CLI still needs one PR on top: the `types.go` pin (Step 3).

**GA promotion after an RC** (`v0.7.0-rc.N` → `v0.7.0`): `release-X.Y` already
exists and already carries the RC pin commit.
- **Do not re-run the branch-creation push** — `main` and the branch have
  diverged, so it fails as non-fast-forward.
- Treat it like a patch bump: PR that merges `main` into `release-X.Y` (skip the
  merge if nothing new on main is wanted), repin `pkg/types.go` from the RC
  version to the GA version, then tag.

**Patch bump** (`0.6.7` → `0.6.8`): the branch already exists and is behind main.
- Open a PR that **merges `main` into `release-X.Y`**. Expect a conflict in
  `pkg/types.go` (`latest` on main vs pinned on the branch) — resolve by pinning
  to the new version.
- **Never `git rebase` a release branch.** A rebase rewrites history and orphans
  previously tagged commits — this already happened: the `v0.6.4-rc.0` and
  `v0.6.4-rc.1` tagged commits are reachable from no branch today. Merge only.

**RC vs GA** differences are collected in [RC specifics](#rc-specifics) below;
the mechanical steps are identical.

## Step 0 — Pre-flight

```bash
# Confirm what is going into the release
git -C <opct> rev-list --count origin/main ^<previous-tag>
git -C <opct-plugins> rev-list --count origin/main ^<previous-tag>
```

Checklist:
- [ ] Jira ticket exists for the release (e.g. OPCT-505) and names the exact version string.
- [ ] Blocking issues triaged — be explicit about what blocks GA vs what blocks the RC. They are different lists.
- [ ] Both repos' `main` is green.
- [ ] You can push branches/tags to both repos. You will **not** have push rights to `openshift/release` — Step 4 is fork-based.
- [ ] Base/tools image has no open Quay Security Scan findings (see below). Do this before tagging — a flagged base image ships into every plugin build on that tag.

**Base image security scan** (carried over from the old devel guide checklist,
not CI-enforced anywhere):

```bash
# build a dev tag and check its Quay scan results
cd opct-plugins && make build-dev   # or: cd opct && make linux-amd64-container IMG=quay.io/my-user/opct && podman push quay.io/my-user/opct:latest
# review results at https://quay.io/repository/opct/opct?tab=tags (or your fork's repo)
```

If the base image has findings:
1. Bump the base image version with the fix (example: [opct-plugins#41](https://github.com/redhat-openshift-ecosystem/opct-plugins/pull/41)).
2. Rebuild and push the tools image: `./hack/build-image.sh build-tools` then
   `podman push quay.io/opct/tools:v0.0.0-<version>`.
3. Re-check the scan; once clean, promote: `podman push quay.io/opct/tools:latest`.
4. Rebuild the plugin images against the new base before continuing to Step 2.

## Step 1 — Not a step: confirm the plan with the maintainer

Get explicit agreement on: version string, whether a new release branch is
needed, and which OCP versions keep a `latest` CI override. These are the three
questions that have required a round-trip on every release so far.

## Step 2 — Plugins repo (`opct-plugins`)

```bash
cd <opct-plugins>
git fetch origin
# minor bump only — seed the branch:
git push origin origin/main:refs/heads/release-0.7
# patch bump — merge main into the branch via PR instead, then pull it.

# Tag the exact reviewed commit by SHA, not local HEAD:
git tag -a v0.7.0-rc.0 -m "Release v0.7.0-rc.0

- OPCT-357: ... (#101)
- ..." <full-sha>
git push origin v0.7.0-rc.0
```

Write the tag message carefully — it is the source material for the GitHub
release notes in Step 5, because the auto-generated changelog is unusable
(see [Known CI bugs](#known-ci-bugs)).

**Two CI runs will appear** when you created a new branch: a branch-push run
(builds, pushes nothing) and the tag-push run. Only the tag run publishes
images. Watch that one:

```bash
gh run list --repo redhat-openshift-ecosystem/opct-plugins --branch v0.7.0-rc.0 --limit 1 --json url --jq '.[0].url'
```

Gate — do not continue until all three exist:

```bash
VER=v0.7.0-rc.0
bad=0
for i in plugin-openshift-tests plugin-artifacts-collector must-gather-monitoring; do
  if tags=$(skopeo list-tags "docker://quay.io/opct/$i" 2>/dev/null) \
     && printf '%s' "$tags" | jq -e --arg v "$VER" '.Tags | index($v)' >/dev/null 2>&1; then
    echo "ok      $i:$VER"
  else
    echo "NOT CONFIRMED $i:$VER (absent, or the registry could not be reached)"; bad=1
  fi
done
[ "$bad" = 0 ] || { echo "gate failed — do not tag the CLI yet"; exit 1; }
```

Two things this gets right that the obvious one-liner does not:

- **Match the tag exactly.** A substring `grep v0.7.0` also matches
  `v0.7.0-rc.0`, so a GA gate would pass while the GA images are absent.
- **Aggregate the result.** A loop exits with the status of its *last*
  iteration, so a `false` on a missing first image is erased by a present third
  one. Track a flag and fail after the loop.
- **Do not distinguish "absent" from "unreachable" here.** Both mean the same
  thing for a gate: you may not tag the CLI yet. (The dead-tag check later *does*
  separate them, because there the two answers lead to opposite decisions.)

RC images never expire (`EXPIRATION: never`), so an RC tag is permanent.

## Step 3 — CLI repo (`opct`)

1. Seed or update `release-X.Y` as decided above.
2. Branch off `release-X.Y`, pin all four constants in `pkg/types.go`:

```go
ControllerImage           = "quay.io/opct/opct:v0.7.0-rc.0"
PluginsImage              = "plugin-openshift-tests:v0.7.0-rc.0"
CollectorImage            = "plugin-artifacts-collector:v0.7.0-rc.0"
MustGatherMonitoringImage = "must-gather-monitoring:v0.7.0-rc.0"
```

   The plugin repo prefix comes from `DefaultToolsRepository`, so plugin
   constants carry no registry host.

3. Open the PR **targeting `release-X.Y`** (release branches are protected;
   direct pushes are not allowed). Title it with the Jira key, e.g.
   `OPCT-505: pin image versions to v0.7.0-rc.0 for release candidate`.
   `verify-images` fails until Step 2's images are on quay — re-run CI after.
4. Merge (merge commit, not rebase), then tag the merge commit by SHA and push:

```bash
git fetch origin && git log --oneline -1 origin/release-0.7   # take the SHA
git tag -a v0.7.0-rc.0 -m "Release v0.7.0-rc.0 ..." <full-sha>
git push origin v0.7.0-rc.0
```

5. Verify what the tag build produced:

```bash
gh run list --repo redhat-openshift-ecosystem/opct --branch v0.7.0-rc.0 --limit 1 --json url,conclusion
skopeo inspect docker://quay.io/opct/opct:v0.7.0-rc.0 >/dev/null && echo image-ok
gh release view v0.7.0-rc.0 --repo redhat-openshift-ecosystem/opct
```

The binary reports `0.0.0+<sha>` for `opct version`. That is expected — the
version is not ldflag-injected (v0.6.7 behaves the same). Not a release defect.

**If the tag build fails**, see [Dead tag recovery](#dead-tag-recovery).

## Step 4 — openshift/release CI bump

CI jobs pin the CLI image. Bump it so the periodics exercise the new release.

Files (3) — all carry `OPCT_CLI_IMAGE` at the top of the workflow:

```
ci-operator/step-registry/opct/conformance/external-aws/opct-conformance-external-aws-workflow.yaml
ci-operator/step-registry/opct/test/platform-none-vsphere/opct-test-platform-none-vsphere-workflow.yaml
ci-operator/step-registry/opct/test/platform-external-vsphere/opct-test-platform-external-vsphere-workflow.yaml
```

Only three files cover every job: AWS has no platform-none workflow (CCM and
non-CCM are variants of the one external-aws workflow), while vSphere splits
none and external into two. Do not hunt for a missing AWS-none file.

Per-version configs under `ci-operator/config/.../*__stable-5.x.yaml` may carry
their own `OPCT_CLI_IMAGE: quay.io/opct/opct:latest` override. Leave those
alone unless the release ticket says otherwise — versions without an override
inherit the workflow default and switch automatically.

Procedure:

```bash
cd <openshift-release>
git checkout -b OPCT-505/opct-rc-v0.7.0 origin/main
# edit the 3 files listed above, then stage them
git add ci-operator/step-registry/opct
git commit -S -s -m "OPCT-505: bump OPCT CLI image to v0.7.0-rc.0"
git push <your-fork> OPCT-505/opct-rc-v0.7.0
gh pr create --repo openshift/release --head <your-gh-user>:OPCT-505/opct-rc-v0.7.0 ...
```

Hard requirements learned the hard way:

- **No push rights to openshift/release.** Push to your fork, PR from
  `<user>:<branch>`.
- **Commits must be both DCO-signed off and cryptographically signed**
  (DPTP-4919). `git commit -S -s`. `Signed-off-by` alone fails
  `hack/verify-commits-signed.py`, which reads GitHub's verification verdict.
  macOS `git log --show-signature` reports "No signature" for SSH signatures —
  it is lying; check `git cat-file commit HEAD` for a `gpgsig` block instead.
- The `commits-signed` job can fail repo-wide on a GitHub API 403 rate-limit,
  with an error that says nothing about signatures. Check whether unrelated PRs
  are failing the same way before debugging your commit; retry with
  `/test commits-signed` after 30–60 min.

Then, in order:

1. `/hold`
2. `/pj-rehearse <job> <job> ...` — cover at least one AWS, one vSphere, and one
   upgrade job. **Re-read `ci-operator/jobs/` for current names before posting**;
   they drift (the old `nightly-4.x` names are now `stable-4.x`; vSphere jobs
   live under `redhat-openshift-ecosystem-opct-main-*`, AWS under
   `openshift-release-main-stable-*`). Names must be space-separated — tabs
   pasted from a table produce "not found to be affected".
3. Rehearse jobs **do not appear in the PR Checks UI** (they are periodic clones
   and report no GitHub status). Find them in the bot comment and prow history.
   That is normal.
4. Any force-push to the branch **aborts in-flight rehearsals** — they run against
   the head SHA at trigger time. Finish amending/signing before triggering.
5. Confirm each rehearsal actually used the new image (grep the init step log).
6. Post a results table comment, then `/pj-rehearse ack` — the merge gate is this
   human ack, not a check. `/hold cancel`.

## Step 5 — GitHub release page

The workflow publishes the release automatically but gets two things wrong
(see [Known CI bugs](#known-ci-bugs)). Fix the body by hand from the tag message:

Step 4 left you in the `openshift/release` checkout — read the tag from the OPCT
clone explicitly, or you will write an empty notes file and wipe the release body:

```bash
git -C <opct> tag -l --format='%(contents)' v0.7.0-rc.0 > /tmp/notes.md
[ -s /tmp/notes.md ] || { echo "empty notes — wrong repo or wrong tag"; exit 1; }
# add an "## Images" section with the 4 quay links, review the file, then:
gh release edit v0.7.0-rc.0 --repo redhat-openshift-ecosystem/opct --notes-file /tmp/notes.md
```

Use `--notes-file`. `gh release edit --notes "$(cat <<EOF ...)"` fails on macOS's
bash 3.2.

Then:
- **GA**: `gh release edit vX.Y.Z --repo redhat-openshift-ecosystem/opct --prerelease=false --latest`.
  Pass `--repo` — without it `gh` targets whatever repo your shell is sitting in,
  and the OPCT release silently stays a pre-release.
- **RC**: change neither flag. `prerelease: true` is hardcoded in the workflow and
  is correct; the Latest badge must stay on the previous GA.

## Step 6 — Validation and close-out

- [ ] Record in the Jira ticket: version, 4 quay image links, release URL, binaries.
- [ ] Watch the affected periodics complete on the new image; link results, file follow-ups.
- [ ] Confirm the versions you deliberately left on `latest` are still green.
- [ ] Keep the release ticket open until periodics have been observed on the new build.
- [ ] For an RC, the GA-promotion work stays on the epic — do not close it.

## RC specifics

- RC is cut from the same branch and with the same mechanics as GA.
- Keep `prerelease: true`; do not touch the Latest badge.
- Plugin pins may legitimately stay at an earlier RC if the plugins were not
  retagged — v0.6.4-rc.1 shipped a CLI-only fix while every image constant
  (including `ControllerImage`) still read `v0.6.4-rc.0`.
- Bumping `openshift/release` to an `-rc` image is now the standard RC flow
  (first done for v0.7.0-rc.0): rehearse the pre-5.0 jobs, leave 5.x on `latest`.

## Dead tag recovery

A tag build can fail before publishing anything. Decision rule:

- **Nothing was consumed** — no quay image, no GitHub release, no downstream
  reference: delete the tag and re-push it on the fixed commit, same version.
  ```bash
  git push origin :refs/tags/v0.7.0-rc.0 && git tag -d v0.7.0-rc.0
  # fix, then re-tag on the new SHA and push
  ```
- **Anything was published or consumed**: never reuse the number. Bump to
  `rc.N+1` (or the next patch).

Check before deciding — the CLI image, all three plugin images, and the release
page. A plugins tag that already published makes the version consumed even if
the CLI build never got that far:

A failed lookup is **not** evidence of absence. Treat a registry error as
"unknown" and stop — never as "safe to reuse":

```bash
VER=v0.7.0-rc.0
REPO=redhat-openshift-ecosystem/opct
published=0; unknown=0

# 1. the four images
for i in opct plugin-openshift-tests plugin-artifacts-collector must-gather-monitoring; do
  if ! tags=$(skopeo list-tags "docker://quay.io/opct/$i" 2>/dev/null) \
     || ! printf '%s' "$tags" \
          | jq -e '(.Tags | type == "array") and all(.Tags[]; type == "string")' \
            >/dev/null 2>&1; then
    echo "UNKNOWN   $i — lookup failed or unexpected response shape"; unknown=1; continue
  fi
  if printf '%s' "$tags" | jq -e --arg v "$VER" '.Tags | index($v)' >/dev/null; then
    echo "PUBLISHED $i:$VER"; published=1
  else
    echo "absent    $i:$VER"
  fi
done

# 2. the GitHub release — a 404 means absent, any other failure means unknown
if out=$(gh api "repos/$REPO/releases/tags/$VER" 2>&1); then
  echo "PUBLISHED github release $VER"; published=1
elif printf '%s' "$out" | grep -q '(HTTP 404)'; then
  echo "absent    github release $VER"
else
  echo "UNKNOWN   github release — $out"; unknown=1
fi

# 3. verdict, only once every check has reported
if   [ "$published" = 1 ]; then echo "=> CONSUMED: bump the version, do not reuse $VER"
elif [ "$unknown"   = 1 ]; then echo "=> UNDECIDED: resolve the errors above before touching $VER"
else                            echo "=> NO PUBLISHED ARTIFACT: confirm nothing downstream references $VER, then delete and re-push"
fi
```

That last verdict is a precondition, not a green light. The script only inspects
the images and the release page; it cannot see downstream use. Before reusing the
number, confirm by hand that no `openshift/release` bump referencing it has
merged, no CI job is pinned to it, and the binaries were not handed to anyone.
Any of those makes the version consumed no matter what the script printed.

## Release-branch hygiene (non-negotiable)

Every fix that lands on a release branch must:

1. land on `main` first, or be explicitly ported there,
2. have a Jira issue, and
3. go through a PR — no direct pushes.

This is not bureaucracy. A Dec 2025 lint hotfix was direct-pushed to
`release-0.6` with no PR and no Jira, never reached `main`, was orphaned by a
history rewrite, and resurfaced a year later as the failing `go_lint` step on
the `v0.7.0-rc.0` tag build.

## Known CI bugs

| # | Symptom | Handling |
|---|---------|----------|
| 1 | `only-new-issues: true` in the lint action has no base commit on a tag build, so it lints the whole repo and pre-existing findings block the release | PR CI now lints the full repository (OPCT-508) so tag builds hold no surprises. If you hit this again, fix the findings — do not add `continue-on-error` |
| 2 | Release body's image link renders `quay.io/opct/opct:` with no tag (reads a nonexistent step output) | Rewrite the body in Step 5 |
| 3 | Auto-changelog groups by PR label; between release tags there is usually only the merge PR, so everything lands in "Uncategorized" | Write the real changelog in the tag message, reuse it in Step 5 |

## Quick reference

```bash
# what's unreleased
git rev-list --count origin/main ^v0.6.7

# tag an exact commit
git tag -a vX.Y.Z -m "Release vX.Y.Z ..." <full-sha> && git push origin vX.Y.Z

# the run that matters is the tag run
gh run list --repo redhat-openshift-ecosystem/opct --branch vX.Y.Z --limit 1 --json url,conclusion

# verify everything published (exact tag match — vX.Y.Z is a prefix of vX.Y.Z-rc.N)
VER=vX.Y.Z
bad=0
for i in opct plugin-openshift-tests plugin-artifacts-collector must-gather-monitoring; do
  if tags=$(skopeo list-tags "docker://quay.io/opct/$i" 2>/dev/null) \
     && printf '%s' "$tags" | jq -e --arg v "$VER" '.Tags | index($v)' >/dev/null 2>&1; then
    echo "ok      $i:$VER"
  else
    echo "NOT CONFIRMED $i:$VER (absent, or the registry could not be reached)"; bad=1
  fi
done
[ "$bad" = 0 ] && echo "all images published" || echo "INCOMPLETE — see above"
gh release view "$VER" --repo redhat-openshift-ecosystem/opct
```

## Related

- `jira-ops` skill — filing and updating the release ticket.
- `ci-triage` skill — triaging the periodics after the bump lands.
- `docs/devel/release.md` — build-pipeline background for the plugin images.
