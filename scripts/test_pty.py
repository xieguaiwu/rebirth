#!/usr/bin/env python3
"""PTY end-to-end test suite for rebirth (stdlib only).

Run: python3 scripts/test_pty.py [path-to-binary]
Covers: full auto-answer flow, step counting, q-quit, Ctrl+C cancel,
pipe mode, seed determinism, CJK garbage input, rapid multi-line paste.
LLM paths are excluded (--no-llm) to keep runs fast and hermetic.
"""
import os, pty
import shutil, subprocess, sys, time, fcntl

BIN = sys.argv[1] if len(sys.argv) > 1 else os.path.expanduser("~/.local/bin/rebirth")
ENV = {k: v for k, v in os.environ.items() if k != "OPENROUTER_API_KEY"}
ENV["XDG_CONFIG_HOME"] = "/tmp/rebirth-pty-config"


def spawn(args):
    master, slave = pty.openpty()
    p = subprocess.Popen([BIN] + args, stdin=slave, stdout=slave,
                         stderr=slave, env=ENV, close_fds=True)
    os.close(slave)
    fcntl.fcntl(master, fcntl.F_SETFL,
                fcntl.fcntl(master, fcntl.F_GETFL) | os.O_NONBLOCK)
    return master, p


def send(m, data):
    os.write(m, data)


def read_all(m, max_wait=1.0):
    out, end = b"", time.time() + max_wait
    while time.time() < end:
        try:
            chunk = os.read(m, 65536)
            if chunk:
                out += chunk
                end = time.time() + 0.15  # extend while data flows
        except BlockingIOError:
            time.sleep(0.01)
        except OSError:
            break
    return out


RESULTS = []


def case(name):
    def deco(fn):
        RESULTS.append((name, fn))
        return fn
    return deco


@case("A_full_flow")
def _(rep):
    m, p = spawn(["--no-llm"])
    out = b""
    for prompt, answer in [("你的出身是", b"1\r"),
                           ("选第 1 个", b"1\r"), ("选第 2 个", b"2\r"),
                           ("选第 3 个", b"3\r"), ("> ", b"\r")]:
        end = time.time() + 10
        while prompt.encode() not in out and time.time() < end:
            out += read_all(m, 0.3)
        send(m, answer)
    # step through up to 120 years
    for _ in range(140):
        out += read_all(m, 0.4)
        if "人生结束".encode() in out or "玩家中途离开".encode() in out:
            break
        send(m, b"\r")
    else:
        p.kill(); return rep(f"never finished: {out[-200:]!r}")
    code = p.wait(timeout=5)
    os.close(m)
    if code != 0:
        return rep(f"exit={code}")
    if "人生结束".encode() not in out and "中途离开".encode() not in out:
        return rep("no ending banner")
    return True


@case("B_step_count")
def _(rep):
    m, p = spawn(["--seed", "31", "--no-llm"])
    out = b""
    for prompt, answer in [("你的出身是", b"1\r"), ("选第 1 个", b"1\r"),
                           ("选第 2 个", b"2\r"), ("选第 3 个", b"3\r"),
                           ("> ", b"\r")]:
        end = time.time() + 10
        while prompt.encode() not in out and time.time() < end:
            out += read_all(m, 0.3)
        send(m, answer)
    read_all(m, 0.5)

    def year_count():
        import re
        return len(re.findall(rb"\[\s*\d+ ", read_and_keep(m)))

    keeper = {"buf": b""}

    def read_and_keep(m_):
        c = read_all(m_, 0.35)
        keeper["buf"] += c
        return keeper["buf"]

    base = year_count()
    for _ in range(4):
        send(m, b"\r")
        time.sleep(0.45)
    after = year_count()
    p.kill(); os.close(m)
    if after - base < 3:
        return rep(f"Enter advanced {after - base} years (base={base})")
    return True


@case("C_q_quit")
def _(rep):
    m, p = spawn(["--no-llm"])
    out = b""
    for prompt, answer in [("你的出身是", b"1\r"), ("选第 1 个", b"1\r"),
                           ("选第 2 个", b"2\r"), ("选第 3 个", b"3\r"),
                           ("> ", b"\r"), ("回车=下一年", b"q\r")]:
        end = time.time() + 10
        while prompt.encode() not in out and time.time() < end:
            out += read_all(m, 0.3)
        send(m, answer)
    out += read_all(m, 2.0)  # drain the post-quit banner
    code = p.wait(timeout=8)
    os.close(m)
    if "血统未记录".encode() not in out and "保持不变".encode() not in out:
        return rep("bloodline note missing")
    if code != 0:
        return rep(f"exit={code}")
    return True


@case("D_ctrl_c_menu")
def _(rep):
    m, p = spawn(["--no-llm"])
    out = b""
    end = time.time() + 10
    while "你的出身是".encode() not in out and time.time() < end:
        out += read_all(m, 0.3)
    send(m, b"\x03")
    try:
        code = p.wait(timeout=6)
    except subprocess.TimeoutExpired:
        p.kill()
        return rep("hung after Ctrl+C")
    os.close(m)
    if code != 0:
        return rep(f"exit={code}")
    return True


@case("E_pipe_mode")
def _(rep):
    inp = b"1\n1\n2\n3\n\n"
    r = subprocess.run([BIN, "--seed", "777", "--auto", "--no-llm"],
                       input=inp, capture_output=True, timeout=30, env=ENV)
    if r.returncode != 0:
        return rep(f"exit={r.returncode} err={r.stderr[:120]!r}")
    if "人生结束".encode() not in r.stdout and "════".encode() not in r.stdout:
        return rep("no game output")
    return True


@case("F_seed_determinism")
def _(rep):
    outs = []
    for _ in range(2):
        shutil.rmtree("/tmp/rebirth-pty-f", ignore_errors=True)
        env_f = dict(ENV, XDG_CONFIG_HOME="/tmp/rebirth-pty-f")
        r = subprocess.run([BIN, "--seed", "42", "--auto", "--no-llm",
                            "--max-age", "40"],
                           input=b"", capture_output=True, timeout=30,
                           env=env_f)
        outs.append(r.stdout)
    if outs[0] != outs[1]:
        return rep("same seed diverged")
    return True


@case("G_cjk_garbage_input")
def _(rep):
    r = subprocess.run([BIN, "--seed", "5", "--auto", "--no-llm"],
                       input="你好世界１２３\n".encode(), capture_output=True,
                       timeout=30, env=ENV)
    bad = "�".encode()
    if bad in r.stdout:
        return rep("replacement char leaked into output")
    if r.returncode != 0:
        return rep(f"exit={r.returncode}")
    return True


@case("H_rapid_paste")
def _(rep):
    # enough Enters to walk any life to its end (max-age 100 needs ~105)
    blob = b"1\r1\r2\r3\r" + b"\r" * 250
    m, p = spawn(["--seed", "9", "--no-llm"])
    send(m, blob)
    out = b""
    end = time.time() + 20
    while time.time() < end:
        out += read_all(m, 0.4)
        if p.poll() is not None:
            break
        if "人生结束".encode() in out:
            break
    alive = p.poll() is None
    finished = "人生结束".encode() in out
    p.kill(); os.close(m)
    if alive and not finished:
        return rep("process hung without finishing")
    if "\ufffd" in out.decode(errors="replace"):
        return rep("replacement char in output")
    if not out:
        return rep("no output at all")
    return True


def main():
    fails = 0
    for name, fn in RESULTS:
        try:
            r = fn(lambda msg: msg)
        except Exception as e:
            r = f"EXCEPTION {e}"
        ok = r is True
        print(f"[{'PASS' if ok else 'FAIL'}] {name}" + ("" if ok else f" — {r}"))
        if not ok:
            fails += 1
    print(f"\n{len(RESULTS) - fails}/{len(RESULTS)} passed")
    sys.exit(1 if fails else 0)


if __name__ == "__main__":
    main()
