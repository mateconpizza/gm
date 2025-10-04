<div align="center">
    <div>
        <h1><b><span style="font-size: 1.2em">📑</span> GoMarks</b></h1>

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/mateconpizza/gm)
![Linux](https://img.shields.io/badge/-Linux-grey?logo=linux)
![SQLite](https://img.shields.io/badge/sqlite-%2307405e.svg?style=Flat&logo=sqlite&logoColor=white)

<img align="center" width="240px" src="https://raw.githubusercontent.com/MariaLetta/free-gophers-pack/refs/heads/master/illustrations/png/19.png" alt="Writing gopher logo">
</div>
<div align="center">
  <sub>💙 Gopher image by <a href="https://github.com/MariaLetta/free-gophers-pack">Maria Letta</a></sub>
</div>
<br>
<span>Simple CLI tool for manage your bookmarks <sub><b>🚧 WIP</b></sub></span>
<br>
<br>
</div>

> [!WARNING]
> This repo is a work in progress!
> Needing both cleaning up and documenting.

https://github.com/user-attachments/assets/b8d8f0fa-e453-421b-b27d-eebb3da7f51f

### Features

- [x] Powered by [`Fzf`](https://github.com/junegunn/fzf)
- [x] Track `bookmarks` with `git` <sub> wip </sub>
  - [x] Sync `bookmarks` as `JSON` files
  - [x] Encrypt `bookmarks` with `GPG` and push to remote
- [x] Encrypt local database with `AES-GCM`
- [x] Support multiple `databases`
- [x] Import `bookmarks` from `firefox` based browsers
- [x] Import `bookmarks` from `chromium` based browsers
- [x] Import `bookmarks` from `git` <sub> wip </sub>
- [x] Fetch `title` and `description` from new bookmark
- [x] Check bookmark `status` _(http status)_
- [x] Support for [`NO_COLOR`](https://no-color.org/) env variable.
- [x] Configure menu `keybinds`, `prompt`, `header`, `preview` _(fzf)_ using a `YAML` file.
- [x] Migrate items from one database to another
- [ ] Add `docker|podman` support <sub>_priority_</sub>

### Installation

```sh
go install github.com/mateconpizza/gm@latest
```

<sub>_To uninstall the program remove the binary in your `go env GOPATH`_</sub>

### Usage <small><sub>(🚧WIP)</sub></small>

```sh
$ gm --help
Simple yet powerful bookmark manager for your terminal

Usage:
  gm [flags]
  gm [command]

Available Commands:
  new         New bookmark, database, backup
  rec         Records management
  tags        Tags management
  db          Database management
  git         Git commands
  io          Export/Import bookmarks
  conf        Configuration management
  help        Help about any command

Flags:
  -c, --copy            copy bookmark URL to clipboard
  -e, --edit            edit bookmark with preferred text editor
  -m, --menu            interactive menu mode using fzf
  -N, --notes           display bookmark notes
  -o, --open            open bookmark in default browser
  -q, --qr              generate QR code for bookmark URL
  -r, --remove          remove bookmark by query or ID
  -f, --field string    output specific field [id|url|title|tags|notes]
  -j, --json            output results in JSON format
  -M, --multiline       output in multiline format (fzf compatible)
  -O, --oneline         output in single line format (fzf compatible)
  -H, --head int        show first N bookmarks
  -t, --tag strings     filter bookmarks by tag(s)
  -T, --tail int        show last N bookmarks
  -S, --snapshot        fetch metadata from Wayback Machine
  -s, --status          check HTTP status of bookmark URLs
  -u, --update          update bookmark metadata
      --color string    output with pretty colors [always|never] (default "always")
      --force           force action | don't ask confirmation
  -n, --name string     database name (default "main.db")
  -v, --verbose count   increase verbosity (-v, -vv, -vvv)
  -h, --help            help for gm
      --version         version for gm
```

### Supported Env Vars

| Name             | type | Description           | Status                                    |
| ---------------- | ---- | --------------------- | ----------------------------------------- |
| `GOMARKS_HOME`   | str  | Path to database/yaml | <span style="color:green">**done**</span> |
| `GOMARKS_EDITOR` | str  | Preferred text editor | <span style="color:green">**done**</span> |
| `NO_COLOR`       | int  | Disable all colors    | <span style="color:green">**done**</span> |

<details>
<summary><strong>📜 Menu support</strong></summary>

Single/multiple selection for open, copy, edit, delete, check status.

https://github.com/user-attachments/assets/b8d8f0fa-e453-421b-b27d-eebb3da7f51f

</details>

<details>
<summary><strong>➕ Add a bookmark</strong></summary>

https://github.com/user-attachments/assets/436b7553-b130-4114-8638-2e8a9b3ea2ce

</details>

<details>
<summary><strong>📝 Edit a bookmark</strong></summary>

https://github.com/user-attachments/assets/059dd578-2257-4db4-b7b1-1267d0375470

</details>

<details>
<summary><strong>🔳 Create QR-Code</strong></summary>

https://github.com/user-attachments/assets/f531fdc9-067b-4747-9f31-4afd5252e3cb

</details>

<details>
<summary><strong>☑️ Check status</strong></summary>

https://github.com/user-attachments/assets/a3fbc64a-87c1-49d6-af48-5c679b1046b1

</details>

<details>
<summary><strong>⚙️ Configuration</strong></summary>

- [x] `$GOMARKS_HOME/config.yml` file

#### YAML file structure

```yaml
prompt: " Gomarks> "
header: true
preview: true
keymaps:
  edit:
    bind: ctrl-e
    description: edit
    enabled: true
    hidden: false
  open:
    bind: ctrl-o
    description: open
    enabled: true
    hidden: false
  preview:
    bind: ctrl-/
    description: toggle-preview
    enabled: true
    hidden: false
  qr:
    bind: ctrl-k
    description: QRcode
    enabled: true
    hidden: false
  toggle_all:
    bind: ctrl-a
    description: toggle-all
    enabled: true
    hidden: true
  yank:
    bind: ctrl-y
    description: yank
    enabled: true
    hidden: false
```

</details>

<details>
<summary><strong>⏳ TODO</strong></summary>

## TODO

### ❗ Priority

- ~~Use a ORM <sub>discontinued</sub>~~
  - [x] Add multiple databases option _(default.db, work.db, client.db)_

### Refactor

- [~] Add `Sync` to remote repo??? (WIP)
- [x] Drop `ErrActionAborted` in package `terminal` (use `sys.ErrActionAborted`)
- [ ] Move `config/menu` to package `menu`
  - [ ] Drop global `Fzf`

#### XDG

- [x] Store `db` in `XDG_DATA_HOME`
- [ ] Store `menu config` in `XDG_CONFIG_HOME` _(WIP: for now in `XDG_DATA_HOME`)_

### 📦 Packages

- [x] `terminal` package
- [x] `color` package
- [x] `files` package

### 🟨 Redo

- [ ] Backups
- [ ] Databases

### ⛓️ Import

- [x] From firefox
  - [ ] If `database` is locked <sub>(SQLITE_BUSY)</sub>, ask user confirmation
        to copy file to `tmp` directory and read from there.
- [x] From ~~chrome~~ chromium

### ♻️ Misc

- ~~Add a logging library~~
- [x] Support `NO_COLOR` env var. [no-color](https://no-color.org/)
- [ ] Create a `rm` subcommand
  - [ ] Add `rm` database, backup
  - [ ] Add `rm` records
  - [ ] Add `rm` misc...
  - [ ] Remove `db rm`

</details>
