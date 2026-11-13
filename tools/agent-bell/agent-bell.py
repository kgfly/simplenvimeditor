#!/usr/bin/env python3
"""Ring the terminal bell (BEL) of the terminal an AI agent CLI runs in.

Used as a task-finished hook by Copilot CLI, Claude Code, code-puppy, OpenAI
Codex and similar CLIs (see docs/agent-bell-setup.md). The bell travels
through tmux and ssh to SimpleNvimEditor, whose attention_on_bell setting
bounces/flashes the Dock/taskbar icon while the window is unfocused.

Along with the BEL it writes OSC 777;snv-attention, which nvim (and tmux's
alert-bell hook) uses to flash the tab number of the nvim tab and the tmux
window holding the agent (docs/agent-bell-setup.md, step 6).

Hook stdout is captured by the CLI, so both are written to the terminal
directly:
  - macOS/Linux: /dev/tty, else the tty of the nearest ancestor process
    (some CLIs run hooks without a controlling terminal). Ancestors come from
    /proc on Linux and ps elsewhere.
  - Windows: the console (ConPTY) of this process, else of the nearest
    ancestor that has one (Node CLIs spawn hooks with a hidden console).
    ConPTY may drop the OSC, so inside an nvim :terminal (no tmux) the
    ancestor pids are also sent to $NVIM to find the agent's terminal.
Always exits 0: a failing Stop hook can block or continue the agent.
"""

import os
import subprocess
import sys

MAX_DEPTH = 64
PAYLOAD = b'\033]777;snv-attention\a\a'


def print_help():
    print('Usage: ' + os.path.basename(__file__) + ' -> ring the bell of the terminal this agent hook runs in')


# ---------------------------------------------------------------- POSIX

def write_bel(path):
    try:
        fd = os.open(path, os.O_WRONLY | os.O_NOCTTY)
    except OSError:
        return False
    try:
        os.write(fd, PAYLOAD)
        return True
    except OSError:
        return False
    finally:
        os.close(fd)


def proc_parent_and_ttys(pid):
    """Linux: parent pid and tty paths of pid's stdio, read from /proc."""
    with open('/proc/%d/stat' % pid) as f:
        stat = f.read()
    # Field 2 (comm) may contain spaces/parens; ppid is the 2nd field after it.
    ppid = int(stat[stat.rindex(')') + 2:].split()[1])
    ttys = []
    for fd in (0, 1, 2):
        try:
            target = os.readlink('/proc/%d/fd/%d' % (pid, fd))
        except OSError:
            continue
        if target.startswith(('/dev/pts/', '/dev/tty')) and target not in ttys:
            ttys.append(target)
    return ppid, ttys


def ps_parent_and_ttys(pid):
    out = subprocess.run(['ps', '-o', 'ppid=,tty=', '-p', str(pid)],
                         capture_output=True, text=True, timeout=2).stdout.split()
    if len(out) < 2:
        raise OSError('ps: no such process')
    tty = out[1]
    return int(out[0]), ([] if tty in ('?', '??', '-') else ['/dev/' + tty])


def ancestor_ttys():
    lookup = proc_parent_and_ttys if os.path.isdir('/proc/self/fd') else ps_parent_and_ttys
    pid = os.getppid()
    seen = set()
    for _ in range(MAX_DEPTH):
        if pid <= 1 or pid in seen:
            return
        seen.add(pid)
        try:
            pid_next, ttys = lookup(pid)
        except (OSError, ValueError, IndexError, subprocess.SubprocessError):
            return
        for tty in ttys:
            yield tty
        pid = pid_next


def posix_bell():
    if write_bel('/dev/tty'):
        return
    for tty in ancestor_ttys():
        if write_bel(tty):
            return


# ---------------------------------------------------------------- Windows

def windows_bell():
    import ctypes
    from ctypes import wintypes

    k32 = ctypes.WinDLL('kernel32', use_last_error=True)
    handle = ctypes.c_void_p
    invalid_handle = handle(-1).value

    class PROCESSENTRY32W(ctypes.Structure):
        _fields_ = [
            ('dwSize', wintypes.DWORD),
            ('cntUsage', wintypes.DWORD),
            ('th32ProcessID', wintypes.DWORD),
            ('th32DefaultHeapID', ctypes.c_size_t),
            ('th32ModuleID', wintypes.DWORD),
            ('cntThreads', wintypes.DWORD),
            ('th32ParentProcessID', wintypes.DWORD),
            ('pcPriClassBase', wintypes.LONG),
            ('dwFlags', wintypes.DWORD),
            ('szExeFile', wintypes.WCHAR * 260),
        ]

    k32.CreateToolhelp32Snapshot.argtypes = [wintypes.DWORD, wintypes.DWORD]
    k32.CreateToolhelp32Snapshot.restype = handle
    k32.Process32FirstW.argtypes = [handle, ctypes.POINTER(PROCESSENTRY32W)]
    k32.Process32FirstW.restype = wintypes.BOOL
    k32.Process32NextW.argtypes = [handle, ctypes.POINTER(PROCESSENTRY32W)]
    k32.Process32NextW.restype = wintypes.BOOL
    k32.CloseHandle.argtypes = [handle]
    k32.CloseHandle.restype = wintypes.BOOL
    k32.FreeConsole.restype = wintypes.BOOL
    k32.AttachConsole.argtypes = [wintypes.DWORD]
    k32.AttachConsole.restype = wintypes.BOOL
    k32.GetConsoleWindow.restype = handle
    k32.CreateFileW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p,
                                wintypes.DWORD, wintypes.DWORD, handle]
    k32.CreateFileW.restype = handle
    k32.WriteFile.argtypes = [handle, ctypes.c_char_p, wintypes.DWORD,
                              ctypes.POINTER(wintypes.DWORD), ctypes.c_void_p]
    k32.WriteFile.restype = wintypes.BOOL

    TH32CS_SNAPPROCESS = 0x2
    GENERIC_READ, GENERIC_WRITE = 0x80000000, 0x40000000
    FILE_SHARE_READ, FILE_SHARE_WRITE = 0x1, 0x2
    OPEN_EXISTING = 3

    def parent_map():
        parents = {}
        snap = k32.CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)
        if not snap or snap == invalid_handle:
            return parents
        try:
            entry = PROCESSENTRY32W()
            entry.dwSize = ctypes.sizeof(PROCESSENTRY32W)
            ok = k32.Process32FirstW(snap, ctypes.byref(entry))
            while ok:
                parents[entry.th32ProcessID] = entry.th32ParentProcessID
                ok = k32.Process32NextW(snap, ctypes.byref(entry))
        finally:
            k32.CloseHandle(snap)
        return parents

    def bell_current_console():
        # A hidden console (CREATE_NO_WINDOW, used by Node for hooks) has no
        # window; a visible or ConPTY console does.
        if not k32.GetConsoleWindow():
            return False
        h = k32.CreateFileW('CONOUT$', GENERIC_READ | GENERIC_WRITE, FILE_SHARE_READ | FILE_SHARE_WRITE,
                            None, OPEN_EXISTING, 0, None)
        if not h or h == invalid_handle:
            return False
        try:
            written = wintypes.DWORD(0)
            return bool(k32.WriteFile(h, PAYLOAD, len(PAYLOAD), ctypes.byref(written), None))
        finally:
            k32.CloseHandle(h)

    parents = parent_map()
    ancestors = []
    pid = parents.get(os.getpid())
    while pid and pid != 4 and pid != os.getpid() and pid not in ancestors and len(ancestors) < MAX_DEPTH:
        ancestors.append(pid)
        pid = parents.get(pid)
    notify_nvim(ancestors)
    if bell_current_console():
        return
    for pid in ancestors:
        k32.FreeConsole()
        if k32.AttachConsole(pid) and bell_current_console():
            return


def notify_nvim(pids):
    """Flag the nvim :terminal whose job is one of pids (see snv_attention_pids)."""
    server = os.environ.get('NVIM')
    if not server or os.environ.get('TMUX') or not pids:
        return
    expr = 'v:lua.snv_attention_pids([%s])' % ','.join(str(p) for p in pids)
    try:
        subprocess.run(['nvim', '--server', server, '--remote-expr', expr],
                       stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                       timeout=3, creationflags=getattr(subprocess, 'CREATE_NO_WINDOW', 0))
    except (OSError, subprocess.SubprocessError):
        pass


def main():
    if len(sys.argv) > 1 and sys.argv[1] == '-h':
        print_help()
        return
    if sys.platform == 'win32':
        windows_bell()
    else:
        posix_bell()


if __name__ == '__main__':
    try:
        main()
    except Exception:
        pass
    sys.exit(0)
