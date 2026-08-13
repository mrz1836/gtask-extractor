<div align="center">

# 📋&nbsp;&nbsp;gtask-extractor

**Export every field of your Google Tasks to JSON — including the hidden metadata the Tasks UI never shows.**

<br/>

<a href="https://github.com/mrz1836/gtask-extractor/releases"><img src="https://img.shields.io/github/release-pre/mrz1836/gtask-extractor?include_prereleases&style=flat-square&logo=github&color=black" alt="Release"></a>
<a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/mrz1836/gtask-extractor?style=flat-square&logo=go&color=00ADD8" alt="Go Version"></a>
<a href="https://github.com/mrz1836/gtask-extractor/blob/master/LICENSE"><img src="https://img.shields.io/github/license/mrz1836/gtask-extractor?style=flat-square&color=blue&v=2" alt="License"></a>

<br/>

<table align="center" border="0">
  <tr>
    <td align="right">
       <code>CI / CD</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/gtask-extractor/actions"><img src="https://img.shields.io/github/actions/workflow/status/mrz1836/gtask-extractor/fortress.yml?branch=master&label=build&logo=github&style=flat-square" alt="Build"></a>
       <a href="https://github.com/mrz1836/gtask-extractor/actions"><img src="https://img.shields.io/github/last-commit/mrz1836/gtask-extractor?style=flat-square&logo=git&logoColor=white&label=last%20update" alt="Last Commit"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Quality</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://codecov.io/gh/mrz1836/gtask-extractor"><img src="https://codecov.io/gh/mrz1836/gtask-extractor/branch/master/graph/badge.svg?style=flat-square" alt="Coverage"></a>
    </td>
  </tr>

  <tr>
    <td align="right">
       <code>Security</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://scorecard.dev/viewer/?uri=github.com/mrz1836/gtask-extractor"><img src="https://api.scorecard.dev/projects/github.com/mrz1836/gtask-extractor/badge?style=flat-square" alt="Scorecard"></a>
       <a href=".github/SECURITY.md"><img src="https://img.shields.io/badge/policy-active-success?style=flat-square&logo=security&logoColor=white" alt="Security"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Community</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/gtask-extractor/graphs/contributors"><img src="https://img.shields.io/github/contributors/mrz1836/gtask-extractor?style=flat-square&color=orange" alt="Contributors"></a>
       <a href="https://mrz1818.com/"><img src="https://img.shields.io/badge/donate-bitcoin-ff9900?style=flat-square&logo=bitcoin" alt="Bitcoin"></a>
    </td>
  </tr>
</table>

</div>

<br/>
<br/>

<div align="center">

### <code>Project Navigation</code>

</div>

<table align="center">
  <tr>
    <td align="center" width="33%">
       🚀&nbsp;<a href="#-installation"><code>Installation</code></a>
    </td>
    <td align="center" width="33%">
       🔑&nbsp;<a href="#-google-cloud-setup-one-time"><code>Google&nbsp;Setup</code></a>
    </td>
    <td align="center" width="33%">
       ⚡&nbsp;<a href="#-quick-start"><code>Quick&nbsp;Start</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
       📚&nbsp;<a href="#-usage"><code>Usage</code></a>
    </td>
    <td align="center">
      🗂️&nbsp;<a href="#-output-format"><code>Output&nbsp;Format</code></a>
    </td>
    <td align="center">
      🔐&nbsp;<a href="#-security"><code>Security</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      🧯&nbsp;<a href="#-troubleshooting"><code>Troubleshooting</code></a>
    </td>
    <td align="center">
      🧪&nbsp;<a href="#-examples--tests"><code>Examples&nbsp;&&nbsp;Tests</code></a>
    </td>
    <td align="center">
      🛠️&nbsp;<a href="#-code-standards"><code>Code&nbsp;Standards</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      🤖&nbsp;<a href="#-ai-usage--assistant-guidelines"><code>AI&nbsp;Usage</code></a>
    </td>
    <td align="center">
       🤝&nbsp;<a href="#-contributing"><code>Contributing</code></a>
    </td>
    <td align="center">
       👥&nbsp;<a href="#-maintainers"><code>Maintainers</code></a>
    </td>
  </tr>
  <tr>
    <td align="center" colspan="3">
       ⚖️&nbsp;<a href="#-license"><code>License</code></a>
    </td>
  </tr>
</table>

<br/>

`gtask-extractor` (binary: `gtasks`) is a small, local, read-only CLI that snapshots a Google
Tasks list to JSON — capturing not just titles and notes but `updated` (which doubles as the
creation time), `position`, `completed`, `hidden`, `deleted`, `parent`, `links`, and
assignment info from Docs/Chat. Fields the raw API drops (`false`, `""`, `null`, `[]`) are all
preserved.

- ✅ **Read-only** — requests only the `tasks.readonly` scope; it can never modify or delete tasks.
- ✅ **Captures every field** — its own JSON envelope preserves values the Google client would drop.
- ✅ **Two modes** — a friendly interactive picker, or a scriptable `gtasks export --list <id> / --all`.
- ✅ **Runs locally** — your data never leaves your machine; there is no server.

```text
$ gtasks
Your task lists:
#  TITLE         UPDATED     ID
1  My Tasks      2026-08-10  MTIzNDU2Nzg5MDEyMzQ1Njc4OTA
2  Work          2026-08-11  Nzg5MDEyMzQ1Njc4OTAxMjM0NTY
3  Sandbox       2026-08-12  OTAxMjM0NTY3ODkwMTIzNDU2Nzg

Select a list to export [1-3] (q to quit): 3
✓ Exported 7 tasks from "Sandbox" → output/sandbox-OTAxMjM0NTY3ODkwMTIzNDU2Nzg-2026-08-12.json
  active: 5   completed: 2   hidden: 1   deleted: 1   assigned: 2
```

<br/>

## 🚀 Installation

Requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy)
(**Go 1.26+** — the tool uses `crypto/rand.Text`, `errors.AsType`, and `reflect.Type.Fields`).

```bash
git clone https://github.com/mrz1836/gtask-extractor.git
cd gtask-extractor
magex build          # → ./bin/gtasks   (or: go build -o gtasks .)
```

> This project uses [**MAGE-X**](https://github.com/mrz1836/mage-x) for builds — there is no
> Makefile. Install it once with `go install github.com/mrz1836/mage-x/cmd/magex@latest`, then
> run `magex help` to see every target. `go build`/`go run` work too.

<details>
<summary><strong>Prebuilt binaries</strong></summary>
<br/>

Once a release is tagged, cross-platform binaries are published via [GoReleaser](.goreleaser.yml)
on the [releases page](https://github.com/mrz1836/gtask-extractor/releases). Download the archive
for your OS/arch, extract the `gtasks` binary onto your `PATH`, and run `gtasks --version`.

</details>

<br/>

## 🔑 Google Cloud Setup (one-time)

Your tasks live in a **personal Google (Gmail) account**. Service accounts and domain-wide
delegation only apply to **Google Workspace** orgs — they cannot read a personal account's tasks.
The only supported path is the **OAuth 2.0 installed-app flow** (loopback redirect + PKCE), so you
need a `credentials.json` describing an OAuth **Desktop app** client. This is a one-time, ~5-minute
setup in the [Google Cloud Console](https://console.cloud.google.com/).

> ⚠️ **Common misconception:** access is **not** granted through `myactivity.google.com`, a
> *Google Search Services Settings* page, or any "Connect Google Workspace" / "search services"
> screen. The **only** place you configure access is the Google Cloud Console, and the **only**
> "allow" you ever click is the browser consent screen this tool opens for you.

1. **Create (or select) a project** at <https://console.cloud.google.com/projectcreate>.
2. **Enable the Google Tasks API**:
   <https://console.cloud.google.com/apis/library/tasks.googleapis.com> → **Enable**.
3. **Configure the OAuth consent screen** (**APIs & Services → OAuth consent screen**, newer UI:
   **Google Auth Platform**):
   - User type / Audience: **External** → **Create**.
   - App name (anything), your email for support + developer contact. Save.
   - **Add yourself as a Test user** — the step that trips people up. Under **Test users** (newer
     UI: **Google Auth Platform → Audience → Test users**) add the **exact Gmail you'll sign in
     with**. In Testing mode only listed testers can authorize; anyone else gets *"Access blocked …
     has not completed the Google verification process"* (Error 403: `access_denied`).
4. **Create the OAuth client ID** (**Credentials → Create credentials → OAuth client ID**, newer
   UI: **Clients → Create client**) → Application type **Desktop app** → **Create**.
5. **Download the JSON** and save it as **`credentials.json`** in the directory where you'll run
   `gtasks`.

`credentials.json` identifies the app; it is not a password, but it is still a secret — it's
git-ignored by this repo.

<br/>

## ⚡ Quick Start

```bash
magex build                         # → ./bin/gtasks
./bin/gtasks                        # opens the browser once for read-only consent
# → pick a list from the numbered menu; JSON lands in ./output/
```

On the **first run** a browser opens to Google's consent screen. Sign in with the Test-user
account → **"Google hasn't verified this app"** → **Advanced → Go to \<your app name\> (unsafe)**
→ approve **"View your tasks"**. The token is cached to `token.json` (0600), so later runs are
silent — no browser. If the browser can't open (e.g. over SSH), the URL is printed to paste
anywhere.

<br/>

## 📚 Usage

`gtasks` has two modes: a friendly **interactive picker** (default) and a scriptable **`export`**
subcommand.

### Interactive (default)

```bash
gtasks
```

Prints a numbered table of your task lists (with IDs); type a number to export one (or `q` to
quit), then it offers to export another. Needs a terminal — if stdin/stdout aren't a TTY it exits
`2` (use `export` below for scripts).

### Non-interactive (`export`) — scripts, cron, re-pulls

```bash
gtasks export --list <id>                 # one list
gtasks export --list <id> --list <id>     # several (repeat --list)
gtasks export --all                       # every task list

# Nightly snapshot of everything into a dated folder
gtasks export --all --output-dir "backups/$(date +%F)"
```

`--list` and `--all` are mutually exclusive; an unknown ID exits `2`. The first run still opens the
browser once; afterward the cached token is reused, so `export` runs **unattended**.

### Flags

Global (apply to every command):

| Flag | Default | Meaning |
|---|---|---|
| `--creds` | `credentials.json` | Path to the OAuth client credentials file |
| `--token` | `token.json` | Path to the cached OAuth token |
| `--output-dir` | `output` | Directory for exported JSON files |
| `--verbose`, `-v` | off | Print extra diagnostic lines (to stderr) |
| `--version` | | Print the version and exit |
| `--help`, `-h` | | Show help (works on subcommands too) |

Subcommands: `gtasks export …`, `gtasks version`.

The machine-facing result (`✓ Exported N tasks … → <path>`) goes to **stdout**; prompts, progress,
and the counts breakdown go to **stderr** — so `gtasks export --list <id> >paths.txt` captures just
the file paths.

**Exit codes:** `0` success · `2` usage / not a TTY / bad flags · `3` credentials or consent
problem · `4` API/network failure · `5` failed to write output · `130` interrupted (Ctrl-C).

<br/>

## 🗂️ Output Format

Each export is a single JSON document. **Every field is always present** — a task that was never
completed still has `"completed": null`, a top-level task still has `"parent": ""`, and so on. The
raw Google API omits these; capturing them is the whole point.

<details>
<summary><strong><code>Field reference</code></strong></summary>
<br/>

| Field | Type | Notes |
|---|---|---|
| `kind` | string | Always `tasks#task` / `tasks#taskList`. |
| `id` | string | Stable task/list identifier. |
| `etag` | string | Version tag of the resource. |
| `title` | string | The task/list title. |
| `notes` | string | Free-text notes. Empty string if none. |
| `status` | string | `needsAction` or `completed`. |
| `updated` | string (RFC 3339) | Last modification time. **If a task was never edited, this is effectively its creation time** — the closest thing the API exposes to "created at". |
| `selfLink` | string | API URL of the resource. |
| `parent` | string | Parent task id; `""` for a top-level task. **`parent` + `position` reconstruct the subtask hierarchy.** |
| `position` | string | Lexicographic manual sort key among siblings. |
| `due` | string (RFC 3339) | Scheduled day. **Date-only** — the time is always `00:00:00Z`; the API cannot store a due *time*. |
| `completed` | string \| null | Completion timestamp, or `null` if not completed. |
| `deleted` | bool | `true` if the task is in the trash. |
| `hidden` | bool | `true` if it was hidden by "clear completed" on the list. |
| `links` | array | Related links (`type`, `description`, `link`); `[]` if none. |
| `webViewLink` | string | Link to the task in the Tasks web UI. |
| `assignmentInfo` | object \| null | Present for tasks assigned from Google **Docs** or **Chat Spaces**; `null` otherwise. Contains `linkToTask`, `surfaceType`, and either `driveResourceInfo` (Docs) or `spaceInfo` (Chat). |

The document also includes an `export` block (generation time, tool version, scope, list id/title,
and a `counts` breakdown) and a `list` block with the six task-list fields.

</details>

<details>
<summary><strong><code>Sample (redacted)</code></strong></summary>
<br/>

```json
{
  "schemaVersion": "1.0",
  "export": {
    "generatedAt": "2026-08-12T15:04:05Z",
    "tool": "gtasks",
    "toolVersion": "1.0.0",
    "scope": "https://www.googleapis.com/auth/tasks.readonly",
    "listId": "MDEyMzQ1",
    "listTitle": "Sandbox",
    "counts": { "total": 7, "needsAction": 5, "completed": 2, "deleted": 1, "hidden": 1, "assigned": 2, "subtasks": 1, "topLevel": 6 }
  },
  "list": {
    "kind": "tasks#taskList",
    "id": "MDEyMzQ1",
    "etag": "\"etag-list\"",
    "title": "Sandbox",
    "updated": "2026-08-01T12:00:00.000Z",
    "selfLink": "https://tasks.googleapis.com/tasks/v1/users/@me/lists/MDEyMzQ1"
  },
  "tasks": [
    {
      "kind": "tasks#task",
      "id": "t1",
      "etag": "\"etag1\"",
      "title": "Buy groceries",
      "notes": "milk & eggs",
      "status": "needsAction",
      "updated": "2026-08-02T10:00:00.000Z",
      "selfLink": "https://tasks.googleapis.com/tasks/v1/lists/MDEyMzQ1/tasks/t1",
      "parent": "",
      "position": "00000000000000000000",
      "due": "2026-08-10T00:00:00.000Z",
      "completed": null,
      "deleted": false,
      "hidden": false,
      "links": [],
      "webViewLink": "https://tasks.google.com/task/t1",
      "assignmentInfo": null
    }
  ]
}
```

Note how `completed`, `parent`, `deleted`, `hidden`, and `links` are all present even when
"empty" — that's the point.

</details>

<br/>

## 🔐 Security

- **Read-only.** Requests only `https://www.googleapis.com/auth/tasks.readonly`. It cannot create,
  edit, or delete tasks.
- **Local only.** All processing happens on your machine; nothing is uploaded anywhere.
- **Secrets stay put.** `credentials.json` and `token.json` are git-ignored; the token cache the
  tool writes (`token.json`) is created atomically with `0600` permissions. Exported data under
  `output/` is git-ignored too.
- **Revoke any time** at <https://myaccount.google.com/permissions>. To reset locally, delete
  `token.json`.
- **Minimal, trusted dependencies** — the official Google API client, the Go team's
  `golang.org/x/oauth2` and `golang.org/x/term`, and Cobra. No heavyweight TUI or browser-launching
  third-party packages.

Read the full [security policy](.github/SECURITY.md).

<br/>

## 🧯 Troubleshooting

| Symptom | Cause & fix |
|---|---|
| **"Access blocked … has not completed the Google verification process"** / `access_denied` (Error 403) after sign-in | The account you signed in with isn't an approved **Test user**. Add that exact Gmail under **Google Cloud Console → Google Auth Platform → Audience → Test users** and re-run. A new test-user entry can take a couple of minutes to propagate. Not a tool bug. |
| **"Google hasn't verified this app"** | Expected in Testing mode. **Advanced → Go to \<your app name\> (unsafe)**. |
| `invalid_grant` on a later run | Cached token is stale/expired. Delete `token.json` and re-run. |
| Auth stops working after ~a week | In **Testing** mode Google expires refresh tokens after ~7 days. Delete `token.json` and re-consent (or publish the app to **Production** to stop the expiry). |
| `redirect_uri_mismatch` | Your OAuth client is the wrong type — it must be **Desktop app**. |
| `403` / "Tasks API has not been used…" | Enable the **Tasks API**: <https://console.cloud.google.com/apis/library/tasks.googleapis.com>. |
| `credentials.json not found` | Download the OAuth **Desktop app** JSON and save it as `credentials.json` (or pass `--creds <path>`). |
| `nothing to export` / `task list not found` | Pass `--list <id>` (repeatable) or `--all`; get IDs from the interactive picker or `gtasks export --all`. |
| "must be run in a terminal" | Interactive mode needs a TTY — use `gtasks export --list <id>` / `--all` for scripts and pipes. |

<br/>

## 🧪 Examples & Tests

All unit tests run via [GitHub Actions](https://github.com/mrz1836/gtask-extractor/actions) on
**Go 1.26.x** and make **zero network requests** — the Google API is exercised through an in-process
`httptest` server, the OAuth flow through a mocked browser + fake token endpoint, and the exporter
through a fake task lister.

```bash
magex test           # run all tests (fast)
magex test:race      # run tests with the race detector
magex test:coverage  # run tests with coverage
magex lint           # golangci-lint v2 (.golangci.json) + go vet
```

Coverage reports are uploaded to [Codecov](https://codecov.io/gh/mrz1836/gtask-extractor) on every
commit.

<details>
<summary><strong><code>Build Commands</code></strong></summary>
<br/>

View all build commands:

```bash script
magex help
```

Common commands:
- `magex build` — build `./bin/gtasks`
- `magex test` — run the test suite
- `magex lint` — run all linters
- `magex deps:update` — update dependencies

</details>

<details>
<summary><strong><code>GitHub Workflows</code></strong></summary>
<br/>

This project uses the **Fortress** workflow system for CI/CD:

- **fortress-test-suite.yml** — full test suite across Go versions
- **fortress-code-quality.yml** — gofmt / golangci-lint / staticcheck
- **fortress-security-scans.yml** — vulnerability scanning
- **fortress-coverage.yml** — coverage reporting to Codecov
- **fortress-release.yml** — automated binary releases via GoReleaser

See all workflows in [`.github/workflows/`](.github/workflows/).

</details>

<details>
<summary><strong><code>Repository Layout</code></strong></summary>
<br/>

```text
.
├── main.go                     # thin entrypoint: os.Exit(cmd.Execute())
├── cmd/                        # Cobra tree: interactive root, `export`, `version`; exit-code mapping
└── internal/
    ├── auth/                   # OAuth loopback + PKCE, token cache, persisting refresh
    ├── tasksclient/            # paginating wrapper around the Tasks API (generic paginate; interface)
    ├── export/                 # JSON envelope, converters, atomic writer, filename slug
    ├── ui/                     # numbered list table + stdin picker (stdlib only)
    └── buildinfo/              # ldflags-injected version
```

A reflection test (`TestNoDroppedFields`) walks the JSON tags of every `tasks.*` type the exporter
mirrors and fails if a field is unmapped — so a future Google client bump that adds a task field
makes the tests fail until the new field is captured.

</details>

<br/>

## 🛠️ Code Standards

Read more about this Go project's [code standards](.github/CODE_STANDARDS.md).

<br/>

## 🤖 AI Usage & Assistant Guidelines

Read the [AI Usage & Assistant Guidelines](.github/CLAUDE.md) for details on how AI is used in this
project and how to interact with AI assistants.

<br/>

## 🤝 Contributing

View the [contributing guidelines](.github/CONTRIBUTING.md) and please follow the
[code of conduct](.github/CODE_OF_CONDUCT.md).

### How can I help?

All kinds of contributions are welcome :raised_hands:! The most basic way to show your support is
to star :star2: the project, or to raise issues :speech_balloon:. You can also support this project
by [becoming a sponsor on GitHub](https://github.com/sponsors/mrz1836) :clap: or by making a
[**bitcoin donation**](https://mrz1818.com/?tab=tips&utm_source=github&utm_medium=sponsor-link&utm_campaign=gtask-extractor&utm_term=gtask-extractor&utm_content=gtask-extractor)
to ensure this journey continues indefinitely! :rocket:

[![Stars](https://img.shields.io/github/stars/mrz1836/gtask-extractor?label=Please%20like%20us&style=social)](https://github.com/mrz1836/gtask-extractor/stargazers)

<br/>

## 👥 Maintainers

| [<img src="https://github.com/mrz1836.png" height="50" alt="MrZ" />](https://github.com/mrz1836) |
|:------------------------------------------------------------------------------------------------:|
|                                [MrZ](https://github.com/mrz1836)                                 |

<br/>

## ⚖️ License

[![License](https://img.shields.io/github/license/mrz1836/gtask-extractor?style=flat-square&color=blue&v=2)](LICENSE)

This project is licensed under the terms of the MIT license. See [LICENSE](LICENSE).
