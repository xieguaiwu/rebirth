[**中文版**](README_zh.md) | **English**

# rebirth — Terminal Life Restart Simulator

A terminal life-restart simulator in Go, inspired by 人生重开模拟器, rebuilt around a computational-psychiatry model of traumatic memory.

## Highlights

- **Trauma dynamics core** — leaky-integrator memory trace, amygdala/prefrontal coupling, reconsolidation feedback, and a saddle-node bifurcation with hysteresis: entering the pathological attractor is easy under sustained adversity, leaving it requires dropping below a *lower* threshold (treatment resistance, simulated).
- **Autocorrelated fortune** — an AR(1) luck process replaces i.i.d. rolls: good years cluster and disasters cluster.
- **Intergenerational bloodline** — each run inherits a sub-additive fraction of the previous life's stress sensitivity (`psi < 1`), so trauma echoes across generations unless interrupted.
- **LLM narration layer (optional)** — `stealth/ox-alpha` via OpenRouter rewrites key events, invents schema-validated "fate events", and writes epitaphs. All RNG stays deterministic; every LLM output is clamped and validated, failures fall back silently.
- **24 careers, 12 births, 200+ events** — from farmer to CEO to digital nomad, sex worker, cult leader (requires high trauma load). Street-level stories inspired by Chinese documentary storytellers.

## Install & Play

```bash
go build -o ~/.local/bin/rebirth .
rebirth                # interactive
rebirth --seed 42 --auto --no-llm   # deterministic auto run
```

| Flag | Effect |
|---|---|
| `--seed N` | fixed seed |
| `--auto` | auto-pick all choices |
| `--no-llm` | disable LLM layer |
| `--model NAME` | OpenRouter model (default `stealth/ox-alpha`) |

Set `OPENROUTER_API_KEY` to enable narration. Without it the game runs fully offline.

## Testing

```bash
go test ./...
```

Covers hysteresis asymmetry, bounded shock dynamics, sub-additive inheritance, seed determinism, career gating, CJK rune-safe editing, and LLM fallback paths.

## License

MIT
