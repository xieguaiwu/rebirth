[**中文版**](README_zh.md) | **English**

# rebirth — Terminal Life Restart Simulator

A terminal life-restart simulator in Go, inspired by 人生重开模拟器 and rebuilt around a computational-psychiatry model of traumatic memory.

```
════ 第 1 代 · 种子 20260823 ════
[出身] 贫民窟 —— 铁皮屋顶下的童年，暴力和匮乏是日常背景音。
[血脉] 应激敏感性基线 0.15（高于此值更易受创）
[天赋] 敏感脆弱 —— 智力+1，但创伤强度×1.5。
[  7 岁] 高年级的孩子堵在巷口，你学会了绕远路回家。
[ 13 岁] 摇晃停止后，你在广场上睡了半个月。人离得很近，心也是。
[ 16 岁] ★ 入行：工厂工人 —— 流水线上的日复一日，汗水换温饱。
──── 人生结束：54 岁 · 职业：工厂工人 · 长期抑郁 ────
墓志铭：一生至此。
[OK] 第 2 代已记录（遗传敏感性 0.76）。
```

## Why this one is different

- **Trauma is a dynamical system, not a debuff list.** A leaky-integrator memory trace couples with amygdala reactivity and prefrontal control. Sustained adversity can push the system past a saddle-node bifurcation into a pathological attractor — and hysteresis means recovery requires dropping below a *lower* threshold than the one that trapped you. Treatment resistance, simulated.
- **Fate autocorrelates.** An AR(1) luck process replaces i.i.d. rolls: good years cluster, disasters cluster, and rags-to-riches arcs need sustained fortune, not one lucky roll.
- **Trauma echoes across generations.** Each run inherits a sub-additive fraction (`ψ = 0.7`) of the previous life's stress sensitivity — and since v0.9.0 that trait genuinely reshapes the next life: it lowers the pathological threshold, skews event sampling toward trauma and away from healing, and slows self-healing. Lineages heal slowly or spiral — your choice of life writes the family's baseline.
- **The LLM never rolls dice.** All randomness lives in the deterministic core. The optional model layer (DeepSeek `deepseek-v4-flash` by default; OpenRouter also supported) only rewrites narration, proposes schema-validated "fate events" (values clamped), and writes epitaphs. Every output is control-character-sanitized before it touches your terminal. A failure breaker stops retrying dead/slow channels: after 3 consecutive failures the rest of the life runs fully local, instantly.
- **Content breadth**: 63 talents across four rarity tiers, 26 careers (farmer → CEO → digital nomad → cult leader* → monk), 13 birth backgrounds (slum to dynasty to war zone), 339 events across 9 shards including street-level stories inspired by Chinese documentary storytellers.

\* The cult-leader track unlocks only at trauma load ≥ 0.45: you have to have been through the abyss to preach from it.

> **Content advisory**: mature themes throughout — trauma, mental illness, sex work, sexuality, cult abuse. Fictional, written for adults; no explicit content.

## Install

```bash
go install github.com/xieguaiwu/rebirth@latest
# or from source:
git clone https://github.com/xieguaiwu/rebirth && cd rebirth
go build -o ~/.local/bin/rebirth .
```

Requires Go 1.25+. Runs fully offline without configuration.

## Usage

```bash
rebirth                          # interactive
rebirth --seed 42 --auto --no-llm   # deterministic auto run (great for CI)
```

| Flag | Effect |
|---|---|
| `--seed N` | fixed seed, fully reproducible runs (combine with `--no-llm`; the optional LLM layer is non-deterministic by nature) |
| `--auto` | auto-pick every choice |
| `--no-llm` | disable the LLM layer |
| `--step` | force manual advance (Enter per year; default on interactive TTY) |
| `--provider NAME` | LLM endpoint preset: `deepseek` (default, direct CN access) or `openrouter` (needs proxy + credits) |
| `--model NAME` | model name (defaults per provider: `deepseek-v4-flash` / `stealth/ox-alpha`) |
| `--llm-url URL` | override the provider base URL (any OpenAI-compatible endpoint) |

Set `DEEPSEEK_API_KEY` (default provider), `OPENROUTER_API_KEY` (with
`--provider openrouter`), or the generic `LLM_API_KEY` to enable narration.
Channels that repeatedly fail (dead, quota-exhausted, or slower than the
12/18s call timeouts) trip a breaker — one notice line, then the rest of
the life plays instantly in fully local mode.
Your bloodline save lives in `~/.config/rebirth/bloodline.json`; delete it
to start a fresh lineage.

### Optional config file

`~/.config/rebirth/config.json` persists your defaults — command-line
flags always win, then this file, then built-ins. Every field is optional:

```json
{
  "provider": "deepseek",
  "model": "",
  "llm_url": "",
  "llm_calls": 24,
  "narrate_ratio": 0.5,
  "max_age": 100,
  "seed": 0,
  "step": false,
  "hints": true,
  "trauma": {
    "enter_at": 0.80,
    "exit_at": 0.35,
    "drive": 0.65,
    "event_trauma_scale": 0.5
  }
}
```

- `llm_calls`: per-life LLM call budget (narration + fate events). The
  epitaph is exempt so it is never starved.
- `narrate_ratio`: fraction of trauma/good events sent to the narrator
  (0.5 = half, deterministic per event ID).
- `trauma.*`: the pathological-attractor dynamics overrides — the
  v0.7.2-calibrated defaults (pathological rate ~28%) are listed; tune
  `enter_at` up for rarer trauma, `drive` down for gentler lives.
- Unknown keys are rejected with a warning (typos fail loudly).

## Project layout

```
main.go                 entry: flags, birth/talent/stat-point flow
internal/game/
  trauma.go             coupled ODE core + hysteresis + inheritance kernel
  events.go             weighted event sampling, talents, bloodline save
  career.go             career tracks + birth backgrounds
  run.go                main loop, death checks, narrator hooks
internal/llm/           provider presets (deepseek/openrouter), budgeted + circuit-broken fail-soft narrator
internal/tui/           zero-dependency rune-safe line editor
internal/game/data/     events_*.json × 9 shards, careers, births, talents
```

## Development

```bash
go vet ./... && go test ./...
go build -o ~/.local/bin/rebirth .   # deploy locally after changes
```

Tests cover hysteresis asymmetry, bounded shock dynamics, sub-additive
inheritance, seed determinism, career gating (including the trauma-gated
cult track), CJK-safe editing, chunk-boundary input parsing, and LLM
fallback paths.

## License

[MIT](LICENSE)
