<div align="center">
    <div>
        <h1><b><span style="font-size: 1.2em">📑</span> GoMarks</b></h1>

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/mateconpizza/gm)
![Linux](https://img.shields.io/badge/-Linux-grey?logo=linux)
![SQLite](https://img.shields.io/badge/sqlite-%2307405e.svg?style=Flat&logo=sqlite&logoColor=white)
![Release](https://img.shields.io/github/v/release/mateconpizza/gm)
![Go Report Card](https://goreportcard.com/badge/github.com/mateconpizza/gm)
![GoDoc](https://pkg.go.dev/badge/github.com/mateconpizza/gm.svg)

<img align="center" width="240px" src="https://raw.githubusercontent.com/MariaLetta/free-gophers-pack/refs/heads/master/illustrations/png/19.png" alt="Writing gopher logo">
</div>
<div align="center">
  <sub>💙 Gopher image by <a href="https://github.com/MariaLetta/free-gophers-pack">Maria Letta</a></sub>
</div>
<br>
<span>Simple CLI tool for manage your bookmarks</span>
<br>
<br>
</div>

> [!WARNING]
> This repo is a work in progress!
> Needing both cleaning up and documenting.

https://github.com/user-attachments/assets/b8d8f0fa-e453-421b-b27d-eebb3da7f51f

### Features

- [x] Powered by [`Fzf`](https://github.com/junegunn/fzf)
- [x] Track bookmarks with `git`
  - [x] `Sync` bookmarks as `JSON` files and push to a remote
  - [x] `Encrypt` bookmarks with [`GPG`](https://gnupg.org/) and push to a remote
  - [x] `Import` from `git`
- [x] Encrypt the local database with `AES-GCM`
- [x] Support multiple `databases`
- [x] Support for `backups`
- [x] Generate `QR-Code`
- [x] Import from `Firefox-based` browsers
- [x] Import from `Chromium-based` browsers
- [x] Fetch titles, descriptions, and keywords
- [x] Check bookmark _(HTTP)_ status
- [x] Clean unnecessary URL `parameters`
- [x] Fetch latest snapshot from the `Wayback Machine`
- [x] Support the `NO_COLOR` environment variable
- [x] Configure [`Fzf`](https://github.com/junegunn/fzf) keybindings, prompt, header, and preview using a `YAML` file
- [ ] Add `docker|podman` support <sub>_priority_</sub>

### Installation

```sh
go install github.com/mateconpizza/gm@latest
```

<sub>_To uninstall the program remove the binary in your `go env GOPATH`_</sub>

### Usage

```sh
$ gm --help
usage: gm [query] [flags] [command]

commands:
  add         add a bookmark
  edit        edit bookmark
  rm          remove bookmark
  open        open in browser
  yank        copy URL
  notes       view notes
  qr          generate QR
  url         URL operations
  tag         tags operations (wip)
  db          database operations
  git         git operations
  config      configuration

flags:
  -t, --tag strings     filter by tag(s)
  -H, --head int        limit to first N bookmarks
  -T, --tail int        limit to last N bookmarks
  -m, --menu            select interactively
  -o, --output string   output format: bar, brief, card, flow, mini, minimal, multiline, oneline
  -s, --sort string     sort by: favorite, newest, visited, popular, reverse
  -f, --fields string   select fields: id, url, title, tags, desc
      --db string       database name (default "main.db")
      --color string    colorize output: always, never (default "always")
  -y, --yes             assume yes
      --force           force action
  -v, --verbose count   increase verbosity (-v, -vv, -vvv)
  -V, --version         version
```

### Environment Variables

<table>
  <tr>
    <td><strong>$GOMARKS_HOME</strong></td>
    <td>
      Path to the data directory containing the databases, backups and configuration file.
      <br />
      <em>If not specified, the default application data directory $XDG_DATA_HOME will be used.</em>
    </td>
  </tr>
  <tr>
    <td><strong>$GOMARKS_EDITOR</strong></td>
    <td>
      A program used to edit text content, e.g. <strong>vim</strong>, <strong>emacs</strong>, <strong>nano</strong>, <strong>gedit</strong>.
      <br />
      <em>If not specified, will fall back to $EDITOR.</em>
    </td>
  </tr>
  <tr>
    <td><strong>$NO_COLOR</strong></td>
    <td>
      Disable all colored output.
      <br />
      <em>If set to a non-empty value, colorized terminal output will be disabled.</em>
    </td>
  </tr>
</table>

### Configuration

<details>
<summary><strong>YAML file structure</strong></summary>

```yaml
db: main
menu:
  defaults: true
  prompt: "▶ "
  preview: true
  header:
    enabled: true
    separator: " / "
  keymaps:
    edit:
      bind: ctrl-e
      description: edit
      enabled: true
      hidden: false
    notes:
      bind: ctrl-w
      description: edit-notes
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
      description: qr-code
      enabled: true
      hidden: false
    open_qr:
      bind: ctrl-l
      description: open-qr
      enabled: true
      hidden: false
    toggle_all:
      bind: ctrl-a
      description: toggle-all
      enabled: true
      hidden: false
    yank:
      bind: ctrl-y
      description: yank
      enabled: true
      hidden: false
```

</details>

### Preview

<details>
<summary><strong>Menu support</strong></summary>

Single/multiple selection for open, copy, edit, delete, check status.

https://github.com/user-attachments/assets/b8d8f0fa-e453-421b-b27d-eebb3da7f51f

</details>

<details>
<summary><strong>Add a bookmark</strong></summary>

https://github.com/user-attachments/assets/436b7553-b130-4114-8638-2e8a9b3ea2ce

</details>

<details>
<summary><strong>Edit a bookmark</strong></summary>

https://github.com/user-attachments/assets/059dd578-2257-4db4-b7b1-1267d0375470

</details>

<details>
<summary><strong>Create QR-Code</strong></summary>

https://github.com/user-attachments/assets/f531fdc9-067b-4747-9f31-4afd5252e3cb

</details>

<details>
<summary><strong>Check status</strong></summary>

https://github.com/user-attachments/assets/a3fbc64a-87c1-49d6-af48-5c679b1046b1

</details>
