<div align="center">
    <div>
        <h1><b><span style="font-size: 1.2em">📑</span> GoMarks</b></h1>

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/haaag/gm)
![Linux](https://img.shields.io/badge/-Linux-grey?logo=linux)
![SQLite](https://img.shields.io/badge/sqlite-%2307405e.svg?style=Flat&logo=sqlite&logoColor=white)

<img align="center" width="240px" src="assets/gophers-19.png" alt="Writing gopher logo">
</div>
<span>Simple CLI tool for manage your bookmarks <sub><b>🚧 WIP</b></sub></span>
<br>
<br>
<div align="center">
  <small>💙 Gopher image by <a href="https://github.com/MariaLetta/free-gophers-pack">Maria Letta</a></small>
</div>
<br>
</div>

> [!WARNING]
> This repo is a work in progress!
> Needing both cleaning up and documenting.

https://github.com/user-attachments/assets/b8d8f0fa-e453-421b-b27d-eebb3da7f51f

### Features

- [x] Powered by [`Fzf`](https://github.com/junegunn/fzf)
- [x] Support multiple `databases`
- [x] Restore `deleted` bookmarks
- [x] Import `bookmarks` from `firefox` based browsers
- [x] Import `bookmarks` from `chromium` based browsers
- [x] Fetch `title` and `description` from added URL
- [x] Check bookmark `status` _(http status)_
- [x] Support for [`NO_COLOR`](https://no-color.org/) env variable.
- [x] Configure menu `keybinds`, `prompt`, `header`, `preview` _(fzf)_ using a `YAML` file.
- [x] Migrate items from one database to another
- [x] Encrypt database <sub>_priority_</sub>
- [ ] Add `docker|podman` support <sub>_priority_</sub>
- [ ] ...

### Installation

```sh
go install github.com/haaag/gm@latest
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
  backup      Backup management
  config      Configuration management
  database    Database management
  help        Help about any command
  import      Import bookmarks from various sources
  new         New bookmark, database, backup
  records     Records management
  remove      Remove databases/backups

Flags:
      --color string    output with pretty colors [always|never] (default "always")
  -c, --copy            copy bookmark to clipboard
  -e, --edit            edit with preferred text editor
  -f, --field string    output by field [id,1|url,2|title,3|tags,4]
      --force           force action | don't ask confirmation
  -H, --head int        the <int> first part of bookmarks
  -h, --help            help for gm
  -j, --json            output in JSON format
  -m, --menu            menu mode (fzf)
  -M, --multiline       output in formatted multiline (fzf)
  -n, --name string     database name (default "bookmarks.db")
  -O, --oneline         output in formatted oneline (fzf)
  -o, --open            open bookmark in default browser
  -q, --qr              generate qr-code
  -r, --remove          remove a bookmarks by query or id
  -s, --status          check bookmarks status
  -t, --tag strings     list by tag
  -T, --tail int        the <int> last part of bookmarks
  -v, --verbose count   Increase verbosity (-v, -vv, -vvv)
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

- [x] `$GOMARKS_HOME/menu.yml` file

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

- [ ] Use a ORM
  - [x] Add multiple databases option _(default.db, work.db, client.db)_
- [ ] Add `Sync` to remote repo???

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

- [ ] From firefox
- [ ] From ~~chrome~~ chromium

### ♻️ Misc

- [ ] Add a logging library
- [x] Support `NO_COLOR` env var. [no-color](https://no-color.org/)

</details>
