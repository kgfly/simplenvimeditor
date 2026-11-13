# Agent bell setup

Run an AI coding agent (GitHub Copilot CLI, Claude Code, code-puppy,
OpenAI Codex, and similar) in a Neovim `:terminal` inside
SimpleNvimEditor, switch to other work, and get pulled back when the
agent finishes or needs an answer:

| OS | What happens | When it stops |
|---|---|---|
| macOS | The Dock icon keeps bouncing and shows a `●` badge | When you switch to the window |
| Windows | The taskbar button flashes, then stays highlighted | When the window comes to the foreground |
| Linux (X11) | The window is marked urgent (taskbar or dock highlight, depending on the desktop) | When you focus the window |
| Linux (Wayland) | An xdg-activation request; most compositors highlight the window | Compositor-defined, usually when you focus it |

Optionally, the Neovim tab and the tmux window that hold the agent flash
too, so you can tell which of many agents is done
([step 6](#6-optional-flash-the-neovim-tab-and-tmux-window)).

- [How it works](#how-it-works)
- [1. Turn on `attention_on_bell`](#1-turn-on-attention_on_bell)
- [2. Let Neovim pass terminal bells through](#2-let-neovim-pass-terminal-bells-through)
- [3. Install `agent-bell.py`](#3-install-agent-bellpy)
- [4. Hook it into your agent CLI](#4-hook-it-into-your-agent-cli)
- [5. tmux and ssh](#5-tmux-and-ssh)
- [6. Optional: flash the Neovim tab and tmux window](#6-optional-flash-the-neovim-tab-and-tmux-window)
- [Troubleshooting](#troubleshooting)

## How it works

```text
agent finishes a task
  → the CLI runs its task-done hook: agent-bell.py
  → agent-bell.py writes a bell (\a) to the agent's terminal
  → (optional) tmux, ssh: the bell travels in the terminal output
  → Neovim :terminal rings the bell (belloff must allow "term")
  → SimpleNvimEditor (attention_on_bell = true) asks the OS for attention
    while its window isn't focused
```

A hook can't just print `\a`: the CLI captures the hook's output, and
some CLIs run hooks without a controlling terminal (macOS, Linux) or with
a hidden console (Windows). [agent-bell.py](../tools/agent-bell/agent-bell.py) finds the
agent's terminal and writes to it directly:

| OS | How it finds the terminal |
|---|---|
| macOS | `/dev/tty`, else the tty of the nearest ancestor process (from `ps`) |
| Linux | `/dev/tty`, else the tty of the nearest ancestor process (from `/proc`) |
| Windows | The nearest ancestor process whose console has a window. That's the ConPTY Neovim created for the `:terminal`. It attaches with `AttachConsole` and writes to `CONOUT$`, and ConPTY forwards the bell to Neovim |

The script needs only Python 3 and its standard library. It always exits
0, because some CLIs treat a failing stop hook as "keep working", and it
never prints anything.

Along with the bell it writes `ESC ] 777;snv-attention BEL`, a marker
that terminals ignore. Step 6 uses it to find which tab or window rang.

## 1. Turn on `attention_on_bell`

Add this to SimpleNvimEditor's `config.toml`, then restart the editor:

```toml
[editor]
attention_on_bell = true
```

| OS | Path |
|---|---|
| Linux | `~/.config/simplenvimeditor/config.toml` |
| macOS | `~/Library/Application Support/simplenvimeditor/config.toml` |
| Windows | `%AppData%\simplenvimeditor\config.toml` |

See [`attention_on_bell`](../README.md#attention_on_bell) in the
README. Nothing happens while the window is focused, since you can
already see it.

## 2. Let Neovim pass terminal bells through

Neovim's default is `belloff=all`, which drops every bell. Remove `term`
from it, for example in `init.lua`:

```lua
vim.opt.belloff = "backspace,cursor,complete,copy,ctrlg,error,esc,ex,hangul,lang,mess,showmatch,operator,register,shell,spell,wildmode"
```

Any other bell that `belloff` lets through (for example `error`) also
triggers attention. Neovim older than 0.11 ignores `belloff` for terminal
bells and always passes them through, so this step is not needed there.

## 3. Install `agent-bell.py`

Install it on the machine where the agent runs. If you ssh to a server
and run the agent there, the script and the hook go on that server.

macOS and Linux:

```sh
mkdir -p ~/bin
curl -fsSL https://raw.githubusercontent.com/kgfly/SimpleNvimEditor/main/tools/agent-bell/agent-bell.py -o ~/bin/agent-bell.py
chmod +x ~/bin/agent-bell.py
```

Windows (PowerShell):

```powershell
New-Item -ItemType Directory -Force "$HOME\bin" | Out-Null
Invoke-WebRequest https://raw.githubusercontent.com/kgfly/SimpleNvimEditor/main/tools/agent-bell/agent-bell.py -OutFile "$HOME\bin\agent-bell.py"
```

On Windows, `python` must be on `PATH` and must be a real Python, not
the Microsoft Store shortcut that opens the Store (install it from
python.org or with `winget install Python.Python.3.13`).

You can also copy [tools/agent-bell/agent-bell.py](../tools/agent-bell/agent-bell.py)
from a clone of this repository. The
examples below assume `~/bin/agent-bell.py` (`%USERPROFILE%\bin\agent-bell.py`
on Windows).

Check it before wiring up any CLI: in a SimpleNvimEditor `:terminal`,
run one of these, then switch to another app within 5 seconds:

```sh
sleep 5; ~/bin/agent-bell.py                          # macOS, Linux
```

```powershell
Start-Sleep 5; python "$HOME/bin/agent-bell.py"       # Windows PowerShell
```

## 4. Hook it into your agent CLI

Hooks are read when the CLI starts, so restart running agent sessions
after adding one.

### GitHub Copilot CLI

Add a user hook file such as `~/.copilot/hooks/agent-bell.json`
(`%USERPROFILE%\.copilot\hooks\agent-bell.json` on Windows). `agentStop`
fires when the agent finishes a turn, and `notification` rings when it
waits for a permission answer. Copilot runs the `bash` field on macOS and
Linux and the `powershell` field on Windows:

```json
{
  "version": 1,
  "hooks": {
    "agentStop": [
      { "type": "command", "bash": "~/bin/agent-bell.py", "powershell": "python \"$HOME/bin/agent-bell.py\"", "timeoutSec": 5 }
    ],
    "notification": [
      { "type": "command", "matcher": "permission_prompt|elicitation_dialog", "bash": "~/bin/agent-bell.py", "powershell": "python \"$HOME/bin/agent-bell.py\"", "timeoutSec": 5 }
    ]
  }
}
```

### Claude Code and other Claude-style hooks

Add a `Stop` hook, for example in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "Stop": [
      { "matcher": "*", "hooks": [ { "type": "command", "command": "~/bin/agent-bell.py" } ] }
    ]
  }
}
```

On Windows these hooks run in Git Bash, or in PowerShell if Git Bash
isn't installed. Use `"command": "python \"$HOME/bin/agent-bell.py\""`
there, which works in both. Add the same hook under `Notification` if
you also want a bell when the agent waits for approval.

### code-puppy

Use the same format in `~/.code_puppy/hooks.json`, but under
`SubagentStop` with the matcher `code-puppy`. Its main agent is named
`code-puppy`, which code-puppy treats as a sub-agent name, so it fires
`SubagentStop` rather than `Stop`:

```json
{
  "hooks": {
    "SubagentStop": [
      { "matcher": "code-puppy", "hooks": [ { "type": "command", "command": "~/bin/agent-bell.py" } ] }
    ]
  }
}
```

On Windows code-puppy runs hooks with `cmd.exe`, so use
`"command": "python \"%USERPROFILE%\\bin\\agent-bell.py\""` there.

### OpenAI Codex

Codex CLI reads Claude-style hooks from `~/.codex/hooks.json`
(`%USERPROFILE%\.codex\hooks.json` on Windows). `Stop` fires when the
agent finishes a turn, and `PermissionRequest` rings when it waits for
approval. The hook prints nothing, so Codex still shows its normal
approval prompt. Codex runs `command` with your shell on macOS and Linux
and `commandWindows` with `cmd.exe` on Windows:

```json
{
  "hooks": {
    "Stop": [
      { "hooks": [ { "type": "command", "command": "~/bin/agent-bell.py", "commandWindows": "python \"%USERPROFILE%\\bin\\agent-bell.py\"", "timeout": 5 } ] }
    ],
    "PermissionRequest": [
      { "hooks": [ { "type": "command", "command": "~/bin/agent-bell.py", "commandWindows": "python \"%USERPROFILE%\\bin\\agent-bell.py\"", "timeout": 5 } ] }
    ]
  }
}
```

The next time Codex starts it asks you to review the new hooks. Choose
**Trust all and continue**; untrusted hooks don't run.

### Other CLIs

Any CLI with a "task finished" hook works the same way: run
`agent-bell.py` from it. If the CLI rings the bell itself (prints `\a`
to its terminal), you don't need a hook at all.

## 5. tmux and ssh

The bell is part of the terminal output, so it gets through ssh and tmux
by itself. For example, SimpleNvimEditor → `:terminal` → ssh → tmux →
agent still reaches the window.

tmux passes a bell on with its defaults: `monitor-bell on`,
`bell-action any`, and `visual-bell off`. If you changed them, set them
back in `~/.tmux.conf` (on the machine running tmux):

```tmux
set -g monitor-bell on
set -g bell-action any
set -g visual-bell off
```

Over ssh, the bell only arrives while the connection is alive. A
connection that dropped silently (sleep, VPN or Wi-Fi change) can look
open locally while the server has already ended the session. Then no
bell gets through, and tmux shows no client (`tmux list-clients` is
empty). To detect a dead connection and close it, add to
`~/.ssh/config` on your local machine:

```sshconfig
Host *
    ServerAliveInterval 30
    ServerAliveCountMax 3
```

## 6. Optional: flash the Neovim tab and tmux window

With several agents in different Neovim tabs or tmux windows, the Dock
or taskbar alone doesn't tell you which one finished. These snippets
flash the matching tab number and tmux window until you visit them.
They use the `snv-attention` marker that `agent-bell.py` writes.

### Neovim tab number

Save this as `~/.config/nvim/plugin/snv-attention.lua`
(`%LOCALAPPDATA%\nvim\plugin\snv-attention.lua` on Windows). It flags
every tab other than the current one that shows the terminal that rang,
and alternates two highlights every 500 ms until you enter the tab:

```lua
-- Flash the tab number of tabs whose :terminal rang agent-bell.py's marker,
-- until you enter the tab.
local api = vim.api
local alerted, timer, phase = {}, nil, true

local function set_hls()
  api.nvim_set_hl(0, 'SnvAttentionA', { fg = '#ffffff', bg = '#d70000', bold = true, default = true })
  api.nvim_set_hl(0, 'SnvAttentionB', { fg = '#000000', bg = '#ffd700', bold = true, default = true })
end
set_hls()

local function any_alerted()
  for tab in pairs(alerted) do
    if not api.nvim_tabpage_is_valid(tab) then alerted[tab] = nil end
  end
  return next(alerted) ~= nil
end

local function tick()
  if any_alerted() then
    phase = not phase
  elseif timer then
    timer:stop()
    timer:close()
    timer = nil
  end
  vim.cmd('redrawtabline')
end

local function alert_buf(buf)
  if not api.nvim_buf_is_valid(buf) then return end
  local cur = api.nvim_get_current_tabpage()
  for _, tab in ipairs(api.nvim_list_tabpages()) do
    if tab ~= cur then
      for _, win in ipairs(api.nvim_tabpage_list_wins(tab)) do
        if api.nvim_win_get_buf(win) == buf then alerted[tab] = true end
      end
    end
  end
  if any_alerted() and not timer then
    phase = true
    timer = vim.uv.new_timer()
    timer:start(500, 500, vim.schedule_wrap(tick))
  end
  vim.cmd('redrawtabline')
end

-- Highlight group for tab number tabnr while it is flagged, else nil.
function _G.snv_attention_hl(tabnr)
  local tab = api.nvim_list_tabpages()[tabnr]
  if tab and alerted[tab] then return phase and 'SnvAttentionA' or 'SnvAttentionB' end
end

-- Called by agent-bell.py on Windows (no tmux) with the hook's ancestor pids.
function _G.snv_attention_pids(pids)
  local wanted = {}
  for _, pid in ipairs(pids or {}) do wanted[tonumber(pid) or -1] = true end
  for _, buf in ipairs(api.nvim_list_bufs()) do
    if vim.bo[buf].buftype == 'terminal' and wanted[vim.b[buf].terminal_job_pid or -2] then
      alert_buf(buf)
    end
  end
  return 0
end

local grp = api.nvim_create_augroup('SnvAttention', { clear = true })
api.nvim_create_autocmd('TermRequest', {
  group = grp,
  callback = function(args)
    local seq = args.data
    if type(seq) == 'table' then seq = seq.sequence end
    if type(seq) == 'string' and seq:find('\027]777;snv-attention', 1, true) then
      vim.schedule(function() alert_buf(args.buf) end)
    end
  end,
})
api.nvim_create_autocmd('TabEnter', {
  group = grp,
  callback = function()
    if alerted[api.nvim_get_current_tabpage()] then
      alerted[api.nvim_get_current_tabpage()] = nil
      tick()
    end
  end,
})
api.nvim_create_autocmd('ColorScheme', { group = grp, callback = set_hls })
```

On Windows, ConPTY may drop the marker. So when the agent runs directly
in a Neovim `:terminal` (no tmux), `agent-bell.py` also calls
`snv_attention_pids()` through `$NVIM` with its ancestor process ids, and
the snippet flags the terminal whose shell is one of them. `nvim` must be
on `PATH` for that. Without this snippet the call does nothing.

To change the colors, define `SnvAttentionA` and `SnvAttentionB` in your
config or colorscheme; the snippet's definitions are only defaults.

The snippet only tracks which tabs are flagged. Your tabline has to draw
them with `snv_attention_hl(tabnr)`. Pick the variant that matches your
setup.

**Built-in tabline** (no tabline plugin). Add to the same file:

```lua
function _G.snv_tabline()
  local s, cur = '', vim.fn.tabpagenr()
  for i = 1, vim.fn.tabpagenr('$') do
    local hl = _G.snv_attention_hl(i) or (i == cur and 'TabLineSel' or 'TabLine')
    local buf = vim.fn.tabpagebuflist(i)[vim.fn.tabpagewinnr(i)]
    local name = vim.fn.fnamemodify(vim.fn.bufname(buf), ':t')
    if name == '' then name = '[No Name]' end
    s = s .. '%' .. i .. 'T%#' .. hl .. '# ' .. i .. ' %#'
      .. (i == cur and 'TabLineSel' or 'TabLine') .. '#' .. name:gsub('%%', '%%%%') .. ' '
  end
  return s .. '%#TabLineFill#%T'
end
vim.o.tabline = '%!v:lua.snv_tabline()'
```

**[bufferline.nvim](https://github.com/akinsho/bufferline.nvim)** (tab
numbers on the right). Run this after `require('bufferline').setup(...)`:

```lua
local tabpages = require('bufferline.tabpages')
local get = tabpages.get
tabpages.get = function(...)
  local tabs = get(...)
  for _, tab in ipairs(tabs) do
    local hl = _G.snv_attention_hl and _G.snv_attention_hl(tab.id)
    local label = tab.component and tab.component[2]
    if hl and label then label.highlight = hl end
  end
  return tabs
end
```

This wraps a bufferline internal, so it may need adjusting after a
bufferline update.

**Other tablines.** Call `snv_attention_hl(tabnr)` while drawing each tab
number and use the group it returns.

### tmux window

tmux drops unknown escape sequences from panes, so the marker doesn't
reach Neovim through tmux by itself. This `alert-bell` hook re-sends it
to every client of the session that rang. That also works back through
ssh. The `window-status-format` flashes a window that rang until you
select it. Add to `~/.tmux.conf` on the machine running tmux, then run
`tmux source ~/.tmux.conf`:

```tmux
# Re-send agent-bell.py's marker to this session's clients (Neovim tab flash).
set-hook -g alert-bell 'run-shell -b "tmux list-clients -t #{q:session_id} -F \"##{client_tty}\" | while read -r t; do printf \"\\033]777;snv-attention\\007\" > \"\$t\"; done"'

# Flash a window that rang (red/yellow each second) until it's selected.
set -g status-interval 1
set -g window-status-bell-style default
set -g window-status-format '#{?window_bell_flag,#{?#{e|m:%S,2},#[bg=red#,fg=white#,bold],#[bg=yellow#,fg=black#,bold]},}#I:#W#{?window_flags,#{window_flags}, }#[default]'
```

The format above is tmux's default window format with the flash added. If
you have your own `window-status-format`, wrap it the same way:
`#{?window_bell_flag,<flash styles>,}<your format>#[default]`.

tmux only flags a window that isn't the current one; if the agent's tmux
window is already current, the Neovim tab flash covers it.

tmux reads `~/.tmux.conf` only when its server starts. Running `tmux`
while any session exists adds a session to the running server and
doesn't read the file again. Run `tmux source ~/.tmux.conf` after every
edit, or quit all sessions (`tmux kill-server`) so the next `tmux`
starts a fresh server.

## Troubleshooting

Work down the chain; each check isolates one link.

1. **Editor.** In a SimpleNvimEditor `:terminal`, run `sleep 5; printf '\a'`
   and switch to another app. Nothing happens?
   - Check `attention_on_bell = true` is under `[editor]` in the right
     `config.toml`, and restart the editor.
   - In Neovim, `:set belloff?` must not contain `all` or `term`.
2. **Script.** Run `sleep 5; ~/bin/agent-bell.py` the same way (on
   Windows, `Start-Sleep 5; python "$HOME/bin/agent-bell.py"`). Step 1
   works but this doesn't?
   - macOS and Linux: is it executable (`chmod +x`), and is `python3` on
     `PATH`?
   - Windows: does `python --version` print a version, not open the
     Microsoft Store?
3. **Hook.** The script works by hand but not from the agent?
   - Restart the agent CLI; hooks are read at startup.
   - code-puppy: the hook must be under `SubagentStop` with matcher
     `code-puppy`, not `Stop`.
   - Copilot CLI: the hook file must end in `.json` and be in
     `~/.copilot/hooks/`; on Windows it needs the `powershell` field.
   - OpenAI Codex: the hooks must be trusted (`/hooks` in Codex lists
     them), and on Windows each needs `commandWindows`.
   - On a remote machine, the script and hook must be on the machine
     where the agent runs.
4. **tmux.** Works without tmux but not inside it? Run
   `tmux show -gw monitor-bell; tmux show -g bell-action; tmux show -g visual-bell`
   and expect `on`, `any`, and `off`.
5. **Tab or window flash** (step 6).
   - Neovim: `:lua print(_G.snv_attention_hl ~= nil)` should print `true`.
     The tab that rang only flashes if it isn't the current tab.
   - tmux: the Dock or taskbar alerts but neither the tmux window nor the
     Neovim tab flashes? The running tmux server probably predates your
     `~/.tmux.conf` edit. `tmux show-hooks -g alert-bell` should list the
     hook and `tmux show -gw window-status-format` should contain
     `window_bell_flag`. If not, run `tmux source ~/.tmux.conf`. Starting
     a new `tmux` session doesn't reload the file.
   - The tab or tmux window flashes but the Dock or taskbar doesn't? The
     bell rang while the editor was still focused. Tabs flash anyway,
     but the window alert is skipped and isn't retried later. This
     happens when a turn ends within a second or two, for example when
     the agent fails at once with an expired login. Check the agent's
     output, then test with `sleep 10; ~/bin/agent-bell.py` and switch
     to another app right away.
   - Remote only: if `tmux list-clients` on the server is empty while
     your local ssh still looks connected, the connection is dead. See
     the ssh keepalive note in step 5.

Platform notes:

- **macOS.** To check whether the badge was set without looking, run
  `lsappinfo info -only StatusLabel <pid>` with SimpleNvimEditor's pid.
- **Windows.** The taskbar button blinks the number of times set by the
  system (`ForegroundFlashCount`), then stays highlighted until you
  switch to the window.
- **Linux (Wayland).** What the request looks like, and whether it shows
  at all, depends on the compositor.
