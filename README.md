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
- **Trauma echoes across generations.** Each run inherits a sub-additive fraction (`ψ = 0.7`) of the previous life's stress sensitivity. Lineages heal slowly or spiral — your choice of life writes the family's baseline.
- **The LLM never rolls dice.** All randomness lives in the deterministic core. The optional model layer (OpenRouter `stealth/ox-alpha`) only rewrites narration, proposes schema-validated "fate events" (values clamped), and writes epitaphs. Every output is control-character-sanitized before it touches your terminal.
- **Content breadth**: 26 careers (farmer → CEO → digital nomad → cult leader* → monk), 12 birth backgrounds (slum to dynasty to war zone), ~300 events across 9 shards including street-level stories inspired by Chinese documentary storytellers.

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
| `--seed N` | fixed seed, fully reproducible runs |
| `--auto` | auto-pick every choice |
| `--no-llm` | disable the LLM layer |
| `--step` | force manual advance (Enter per year; default on interactive TTY) |
| `--model NAME` | OpenRouter model (default `stealth/ox-alpha`) |

Set `OPENROUTER_API_KEY` to enable narration. Your bloodline save lives in
`~/.config/rebirth/bloodline.json`; delete it to start a fresh lineage.

## Project layout

```
main.go                 entry: flags, birth/talent/stat-point flow
internal/game/
  trauma.go             coupled ODE core + hysteresis + inheritance kernel
  events.go             weighted event sampling, talents, bloodline save
  career.go             career tracks + birth backgrounds
  run.go                main loop, death checks, narrator hooks
internal/llm/           OpenRouter client, budgeted fail-soft narrator
internal/tui/           zero-dependency rune-safe line editor
internal/game/data/     events_*.json × 6 shards, careers, births, talents
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
