# wlbrowser

A small Wayland-friendly browser wrapper around WebKitGTK 6.0 with vim-style
keybindings. No tabs, no chrome — just a window, a webview, and a `:` /  `/`
command bar. Bindings are baked in by default; you can override or add new
ones in a TOML config.

## Install

System dependencies (Arch):

```sh
sudo pacman -S webkitgtk-6.0 gtk4 pkgconf
```

(Debian/Ubuntu equivalents: `libwebkitgtk-6.0-dev libgtk-4-dev pkg-config`.)

Build:

```sh
go build -o wlbrowser ./cmd/wlbrowser
```

Optional — seed a config you can edit:

```sh
./wlbrowser --write-config
$EDITOR ~/.config/wlbrowser/config.toml
```

Set as default browser:

```sh
./wlbrowser --set-default
```

Or add `set_default = true` to your `config.toml`.

## Modes

| Mode    | How you get there                                | What keys do                       |
|---------|---------------------------------------------------|------------------------------------|
| Normal  | default                                          | dispatch via the binding table     |
| Insert  | focus an `<input>`, `<textarea>`, contenteditable | pass through; only `Esc` is grabbed |
| Command | `:` (or `o`/`O` shortcuts)                       | the bottom entry takes input       |
| Find    | `/`                                              | live in-page search                |

Insert mode toggles automatically — you don't have to think about it. Press
`Esc` from Insert to blur the input and return to Normal.

## Default bindings (Normal mode)

| Keys      | Action                                       |
|-----------|----------------------------------------------|
| `j` / `k` | scroll line down / up                        |
| `h` / `l` | scroll left / right                          |
| `d` / `u` | scroll half-page down / up                   |
| `gg` / `G`| top / bottom                                 |
| `H` / `L` | history back / forward                       |
| `r` / `R` | reload / hard reload                         |
| `gh`      | go home                                      |
| `+` `=` `-` `0` | zoom in / in / out / reset             |
| `o`       | open command bar primed with `open `         |
| `O`       | open command bar primed with the current URL |
| `/`       | enter find mode                              |
| `n` / `N` | next / previous find match                   |
| `:`       | enter command mode                           |
| `yy`      | copy the current URL to the clipboard         |
| `Esc`     | clear pending sequence / exit Insert         |

Counts work on motions: `5j`, `12k`, `3d`, etc.

## `:` commands

| Command                  | Effect                                              |
|--------------------------|-----------------------------------------------------|
| `:open <url>` / `:o`     | navigate                                            |
| `:back` / `:forward`     | history                                             |
| `:reload`                | reload                                              |
| `:home`                  | jump to configured home                             |
| `:js <script>`           | evaluate arbitrary JavaScript in the current page   |
| `:exec <argv...>`        | run a shell command; `{url}` is substituted         |
| `:set zoom=<float>`      | runtime zoom level                                  |
| `:set home=<url>`        | runtime home (not persisted)                        |
| `:quit` / `:q`           | close the window                                    |

## Config

Path: `$XDG_CONFIG_HOME/wlbrowser/config.toml` (usually `~/.config/wlbrowser/config.toml`).

Layout:

```toml
home   = "https://duckduckgo.com"
title  = "wlbrowser"
width  = 1280
height = 800
seq_timeout_ms = 600   # ms to wait for the next key in an ambiguous sequence

[keys.normal]
"j"  = "scroll-down"
"<C-r>" = "reload"
"<Space>m" = { action = "exec", command = ["mpv", "{url}"] }
"yf" = { action = "exec", command = ["wl-copy"], stdin = "{url}" }
```

Key string syntax (vim-flavored):

- Plain ASCII (`j`, `?`, `/`, `:`)
- Sequences (`gg`, `<Space>m`, `gG`)
- Special tokens: `<Esc>`, `<CR>`, `<Tab>`, `<Space>`, `<BS>`, `<Up>`/`<Down>`/`<Left>`/`<Right>`, `<F1>`–`<F12>`, `<Plus>`, `<Minus>`
- Modifiers: `<C-r>` (Ctrl), `<S-l>` (Shift), `<A-x>` (Alt), `<M-x>` (Super), combinable: `<C-S-r>`

Action values are either bare strings (any built-in name, see the table
below) or tables with at least an `action` key:

```toml
"yu" = { action = "exec", command = ["xdg-open", "{url}"] }
"hj" = { action = "js", script = "alert('hi')" }
"od" = { action = "open-url", url = "https://duckduckgo.com" }
```

### Built-in action names

`back`, `forward`, `reload`, `reload-hard`, `stop`, `home`, `open-url`,
`scroll-up`, `scroll-down`, `scroll-left`, `scroll-right`,
`scroll-half-page-up`, `scroll-half-page-down`, `scroll-top`, `scroll-bottom`,
`zoom-in`, `zoom-out`, `zoom-reset`,
`cmd-mode`, `cmd-open`, `cmd-open-current`,
`find-start`, `find-next`, `find-prev`,
`escape`, `js`, `exec`.

## Out of scope (for now)

Tabs, hint mode (`f`), bookmark/history UI, hot config reload, multiple
windows. Architecture leaves room for each but they're not in v1.
