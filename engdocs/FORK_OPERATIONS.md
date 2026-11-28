# Fork operations

| Version | Date | Author | Change |
|---|---|---|---|
| 1.0 | 2026-11-28 | Marlon Costa <marlon.costa@datacosmos.com.br> | Record branch roles and the upstream mirror credential requirement. |

This runbook covers `marlon-costa-dc/beads`. The fork's development and
validation rules are in [AGENTS.md](../AGENTS.md) and
[TESTING.md](TESTING.md).

## Branch roles

| Branch | Role | Integration policy |
|---|---|---|
| `dc-use` | Default branch and fork integration lane | Feature branches start here and return through reviewed PRs, native checks, and a merge commit. |
| `main` | Mirror of `gastownhall/beads` `main` | Fast-forward only. Import upstream changes into `dc-use`; do not push fork customizations back into this mirror. |
| `legacy-main` | Historical fork integration head | Its content is already represented in `dc-use`; the remaining historical merge can be absorbed without restoring old files. |
| `wip/sweep-preserve/base-green-20261116-a4e84178` | Preserved work from the base repair lane | Already contained in `dc-use` through [PR #78](https://github.com/marlon-costa-dc/beads/pull/78). |
| `__dolt_remote_info__` | Metadata for this repository's Dolt remote | Independent root history; preserve separately from code branches. It references `refs/dolt/data`. |

The branch inventory above was checked on 2026-11-28. A future integration
must re-read the remote heads before acting. Do not use force pushes or
`--allow-unrelated-histories` to combine the Dolt metadata with source code.

## Why the upstream mirror needs a dedicated App

[upstream-sync.yml](../.github/workflows/upstream-sync.yml) mirrors the
upstream branch, including changes under `.github/workflows/`. The ordinary
workflow token's `contents: write` grant does not supply the GitHub App
`Workflows` repository permission needed for that push. GitHub documents
`Contents` for HTTP Git access and `Workflows` for changes to Actions files
in [Choosing permissions for Git access](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/choosing-permissions-for-a-github-app#choosing-permissions-for-git-access).

The workflow requests an installation token restricted to this repository:

| Setting | Required value |
|---|---|
| Installation owner | `marlon-costa-dc` |
| Selected repository | `beads` only |
| Repository permissions | `Contents: Read and write`; `Workflows: Read and write` |
| Actions secret | `UPSTREAM_SYNC_APP_CLIENT_ID`: the App's **Client ID** |
| Actions secret | `UPSTREAM_SYNC_APP_PRIVATE_KEY`: the App's generated PEM private key |

The Client ID is distinct from the numeric App ID. The pinned
[create-github-app-token action inputs](https://github.com/actions/create-github-app-token/blob/bcd2ba49218906704ab6c1aa796996da409d3eb1/action.yml)
declare `client-id`, `permission-contents`, and `permission-workflows`.
The older `app-id` input is deprecated. No autofix credential or token
fallback is used.

## Provision the credential

1. In GitHub's developer settings, register a private GitHub App for the
   account that owns the fork. It needs the two repository permissions in
   the table; no organization, administration, or pull-request permission
   is required for this mirror workflow. Webhook delivery is not needed.
2. Install the App on `marlon-costa-dc`, selecting only the `beads`
   repository. If changing an existing App's permissions, approve the
   updated permissions for this installation too.
3. Generate a private key for the App. In the fork's **Settings → Secrets
   and variables → Actions**, create the two repository secrets above.
   Store the Client ID and PEM key in their respective fields. Do not put
   the key in a commit, PR body, issue, or workflow log.
4. After the updated workflow is present on `dc-use`, open **Actions → Sync
   upstream main → Run workflow** and select `dc-use` as the workflow ref.
   The workflow itself checks out and updates the separate `main` mirror.
5. Inspect the complete run. The credential preflight, token creation,
   fast-forward, push, and remote-head verification must all succeed.
   Record the run URL and mirrored SHA in the integration evidence.

Provisioning is an owner operation. This change **does not provision an
App, install it, create secrets, or prove that a usable credential exists**.
As of this version, the workflow has **not been validated remotely with
the new installation token**. An unset secret produces a clear failure
before the checkout; a misconfigured installation fails when minting or
using the token.

## Execution and verification

The workflow runs daily at 07:37 UTC and supports manual dispatch. Its
ordinary `GITHUB_TOKEN` has read-only contents access and is not persisted
by checkout. The fetch and fast-forward happen before the App token is
created. After token creation, only the inline Git authentication, push,
and remote-head comparison run; no script, action, or build from the
checked-out upstream tree is executed. The token action retains its
default end-of-job revocation.

The mirror update must satisfy both checks:

- After `git merge --ff-only FETCH_HEAD`, local `HEAD` equals the fetched
  upstream SHA.
- After the ordinary push to `refs/heads/main`, that remote ref equals the
  local `HEAD`.

A successful mirror run updates `main` only. Incorporating that code into
`dc-use` remains a separate PR with the fork's quality gates and review.
Do not describe a mirror run as proof that the integrated fork passes CI.

| Failure | Required action |
|---|---|
| Missing `UPSTREAM_SYNC_APP_*` secret | Complete credential provisioning; the workflow deliberately fails. |
| Token creation denied | Check the Client ID/key pair, installation owner, selected repository, and accepted App permissions. |
| Push refuses workflow changes | Confirm the installation has `Workflows: Read and write`, in addition to `Contents: Read and write`. |
| `merge --ff-only` refuses divergence | Inspect and preserve both histories; resolve the mirror policy before rerunning. Do not force the ref. |
| Remote head differs after push | Inspect concurrent branch activity and the run's captured SHAs before attempting another sync. |

Neither credential provisioning nor mirror repair changes branch
protections, CI requirements, release gates, or the integration branch.
