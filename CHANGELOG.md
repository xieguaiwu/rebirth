## [0.9.0] - 2026-08-24

### Changed (inherited trauma sensitivity now genuinely shapes lives)

Math audit (v0.8.x): sensitivity s only shifted the birth-year baseline
(m0 = 0.05+0.30s, a0 = AStar+0.35s). With the amygdala relaxing in
~1.2 years and the extinction term (−0.10/yr) erasing the memory
baseline in ~2–3 years, plus the entry threshold at m ≥ ~0.85 — far
above anything s could supply — the heritable trait was decoration:
24,000 simulated lives showed an identical 35.7% pathological rate for
s = 0..1, and lineage s-chains were just echoes of each generation's own
trauma with zero memory. The README's "trauma echoes across
generations" claim was mathematically false.

v0.9.0 wires s into the dynamics at four points (all per-unit-s,
configurable via TraumaParams):

- **SensEnterAt = 0.30** — inherited sensitivity lowers the pathological
  entry threshold itself (EnterAt −= 0.30·s, hysteresis pair guarded so
  EnterAt never drops to/below ExitAt). At s=1 the threshold falls from
  0.80 to 0.50: two childhood traumas now suffice to enter the
  attractor instead of ~9–11.
- **SensTraumaW = 0.40 / SensHealW = 0.40** — event sampling biases:
  trauma events get heavier, healing events lighter for high-s
  lineages (PickEvent gains the params + sensitivity arguments).
- **SensExtinct = 0.50** — extinction learning slows by (1−0.5·s), so
  sensitive lineages take roughly twice as long to heal on their own.

Measured pathological rate vs inherited sensitivity (3,000 lives each,
identical seed streams, only s differs):

| s | patho | mean age |
|---|---|---|
| 0.00 | 35.8% | 73.0 |
| 0.25 | 50.8% | 67.4 |
| 0.50 | 65.1% | 60.9 |
| 0.75 | 80.0% | 52.0 |
| 1.00 | 94.3% | 41.9 |

(v0.8.x: flat 35.7% at every level.) Baseline s=0 is unchanged, so the
TestPathologicalRateBand balance gate stays green. Lineage chains now
show the intended dynamics: pathological families self-sustain (s
saturates ~0.7, ~80% re-pathologizing), one recovered generation breaks
the chain (s → ~0.01), and renewed trauma re-establishes it.

- New tests: TestGeneticSensitivityMatters (rate must rise strictly
  with s, top-of-range ≥ 8pp above baseline — the old model measured
  exactly 0), TestSensitivitySlowsExtinction, TestSensitivityThresholdShift.

## [0.8.1] - 2026-08-24

### Fixed (ages 1–2 always blank — "二岁的内容总是不出来")

Early childhood pool used to hold exactly two events: `loved_child`
(0–5) and `abandoned` (0–2, requires 家境≤3). Once `loved_child` fired
at 0/1 — which it almost always did — age 2 (and sometimes 1) had zero
eligible events for any player with 家境>3, so the year produced no line.

- Added three unconditional toddler events in `events_01_core.json`:
  `first_words` (1–2, int+0.5/spr+1), `first_steps` (1–2, str+1),
  `picture_book` (2–3, int+1/spr+0.5). Light, no trauma, matches the
  tone of the existing 0–1 events; ages 1–3 now always have ≥3
  unconditional candidates regardless of consumption order. Event
  count 336 → 339.
- **Regression test** `TestEarlyChildhoodNeverSilent`: for every birth
  background × 40 seeds (520 lives), ages 1–3 must produce a line.
  (The first version of the test forgot `WithPoints` and instantly
  killed every run — stats base is 0 without the 20-point allocation;
  caught in one debug pass, not a game bug.)
- Balance gate `TestPathologicalRateBand` still green.

## [0.8.0] - 2026-08-24

### Fixed (44-minute "命运编织中 stuck" report)

Root cause chain (all measured, not guessed):

1. **OpenRouter account had no credits** (HTTP 402 "never purchased
   credits") — every paid model refused; the only usable channel was the
   free `stealth/ox-alpha`.
2. **ox-alpha's real latency (15.2s narrate / 39.5s fate prompt) exceeded
   the game's 12/18s timeouts** — every single call burned the full
   timeout and then failed, forever, with no circuit breaker.
3. Net effect: the game froze ~12s before every sampled event (the dim
   "命运编织中，稍等片刻" hint), silently fell back each time, and crawled
   all life long. Empirically: 90s against a dead endpoint only reached
   age ~12 on v0.7.4.

Changes:

- **Failure breaker in `llm.Narrator`**: after 3 consecutive failures
  (timeout / HTTP error / schema violation) the channel is declared
  broken for the rest of the life; every method returns its fallback
  instantly without touching the network. Any success resets the
  streak (intermittent channels keep working). New `Broken()` on the
  `game.Narrator` interface; `Run` prints a one-time notice:
  `[提示] 叙事通道连续失败（不可用或过慢），本世余下改为纯本地叙事。`
  Post-fix e2e vs the same dead endpoint: full 45-year life in 36s
  (3×12s breaker window) instead of crawling forever.
- **Default provider flipped openrouter → deepseek**: verified working
  (5.2s, valid JSON, direct CN access, no proxy needed); v4-flash is
  fast and cheap, and the whole fate+narrate+epitaph chain was already
  verified on 0.7.4. openrouter stays fully supported via
  `--provider openrouter` / config; note it now needs BOTH a proxy and
  account credits for most models (free stealth channels are unstable).
- **Hardening**: `complete()` no longer falls back to `http.DefaultClient`
  (which has no timeout at all); a nil client gets a 45s-timeout client.
- **Regression tests**: `TestTimeoutsEnforced` (stalled server must fail
  at 12/18s ctx deadlines — proves no call can hang a session),
  `TestBreakerTripsAfterConsecutiveFailures` (instant fail-soft, budget
  untouched), `TestBreakerResetsOnSuccess` (streak resets).
- Real-API smoke: DeepSeek full chain on a 45-year auto life — narration
  polished through age 45, fate event injected at 30, no false breaker
  trips; single epitaph miss fell back cleanly (v4-flash occasionally
  burns max_tokens on reasoning — known, fail-soft by design).

## [0.7.4] - 2026-08-24

### Added (player config file + LLM budget overhaul)
- **Optional config file** `~/.config/rebirth/config.json` (new package
  `internal/config`): provider/model/llm_url/llm_calls/narrate_ratio/
  max_age/seed/step/hints + `trauma` dynamics overrides (enter_at,
  exit_at, drive, event_trauma_scale). Precedence: flags > config >
  defaults (flag.Visit tracks explicit flags). Unknown keys fail loudly
  (DisallowUnknownFields); broken files WARN and fall back to defaults.
- **Trauma dynamics are now tunable at runtime**: Drive and EventScale
  moved from hard-coded constants into TraumaParams (defaults unchanged:
  0.65 / 0.5); clamp01 exported as Clamp01 for config validation.
- **LLM budget overhaul** (real-API finding): the 10-call budget was
  exhausted by narration alone before age 45 — every later event and the
  epitaph silently fell back. Default budget 10 → 24, epitaph is exempt
  from the budget (one call per life, never starved), and narration is
  sampled deterministically per event ID (narrate_ratio, default 0.5).
- **DeepSeek real-API path verified end to end** (first time): with
  DEEPSEEK_API_KEY + --provider deepseek, fate events inject, narration
  samples, epitaph returns a real model line ("最俊的农夫，种了一生沉默").
- Tests: config package (missing file / full parse / unknown-key rejection
  / partial trauma override), narrate sampling determinism + ratio reach,
  custom base-URL override.

## [0.7.3] - 2026-08-24

### Changed (implementation swap + LLM provider settings)
- **Balance fix moved from data to code**: the v0.7.2 event-alpha halving is
  now `game.EventTraumaScale = 0.5`, applied to event shocks at run time.
  All 9 data shards restored to their authored alpha values. 500-seed
  measurement identical (pathological 28.2%, avg age 79.5, early <30
  3.4%); TestPathologicalRateBand gate unchanged (<50%).
- **LLM layer is now provider-configurable**: `--provider openrouter|deepseek`
  presets (base URL + default model + key env), `--llm-url` overrides the
  endpoint (any OpenAI-compatible API), `--model` defaults per provider.
  - openrouter: https://openrouter.ai/api/v1 · `stealth/ox-alpha` ·
    `OPENROUTER_API_KEY`
  - deepseek: https://api.deepseek.com/v1 · `deepseek-v4-flash` ·
    `DEEPSEEK_API_KEY`
  - Generic `LLM_API_KEY` fallback accepted for any provider.
- max_tokens raised (Narrate 300→600, FateEvent 500→900, Epitaph 200→400)
  so DeepSeek V4 reasoning (reasoning_content spends budget before
  content) cannot starve the visible reply.
- New tests: provider preset resolution, custom base-URL override path.

## [0.7.2] - 2026-08-24

### Fixed (audit round — balance calibration + dead state)
- **Pathological attractor re-calibrated** — measured pathological rate was
  83% of lives (diagnostic: 300 seeds), far above the game-scale target;
  CONTEXT documented ~60%. Root cause: the event pool is trauma-saturated
  (160/336 events carry trauma_alpha, mean 0.31) so the memory trace m
  saturated at ~1.0 and Load crossed EnterAt by ages 5-11 in most lives.
  - Event trauma_alpha halved across all 9 shards (160 events; new mean
    0.156, max 0.325) — the documented calibration lever.
  - Amygdala drive 0.9 → 0.65 (memory-driven excitation).
  - EnterAt 0.70 → 0.80 (pathological entry threshold).
  - Result: 28.0% pathological on the 200-seed regression gate
    (TestPathologicalRateBand, threshold < 50%), average death age ~80,
    early (<30) death 3.3%.
- Removed dead `LowStreak` state: written by Shock/Step but never read;
  the extinction gate is arousal (A < 0.55), not a streak counter.
- PTY suite: new case I (line-editing keys on a real PTY — left arrow,
  backspace, insert redraw); test_pty.py now 9 cases.
- TestCultLifeCoherence no longer flags legal career re-entry lines
  (quit → re-enter → quit again produces identical "离开「X」。" lines;
  only event lines must be unique).
- READMEs: event count corrected 340 → 336; `--seed` docs now note
  `--no-llm` is required for full reproducibility.

## [0.7.1] - 2026-08-24

### Fixed (oracle verification round)
- Bloodline inherited-talent is seeded from the previous save (only a
  picked inheritable talent overrides it) — the wipe survived round 1's
  incomplete fix and was empirically reproduced by oracle.
- Orphan "\n" after a chunk-end Enter no longer phantom-submits an empty
  line (carrySkipNL hand-off between calls).
- Ctrl+C inside a half-carried CSI sequence now cancels instead of being
  swallowed as a CSI parameter byte.
- PTY suite: B_step_count pinned to a seed; H_rapid_paste supplies enough
  Enters to finish a life and fails on hangs.

## [0.7.0] - 2026-08-24

### Fixed (momus audit round 2 — all 9 findings, each reproduced before fixing)
- **P1** Quitting mid-life corrupted the lineage save (zero sensitivity +
  generation bump written). Abort now fills a full Result and main skips
  the save entirely.
- **P1** The v0.6.0 shards used a `context` JSON key the Event struct never
  declared — 12 storyline events were unreachable at runtime. Context is
  now an alias of Sets, and LoadEvents decodes with
  DisallowUnknownFields so key drift fails loudly at startup.
- **P1** ReadLineErr dropped feed()'s carried tail between os.Stdin.Read
  chunks and could re-feed identical bytes forever; the loop now blocks
  for the continuation and prepends it (split escapes / CJK runes safe).
- **P2** CRLF paste no longer double-submits; hint is cleared AFTER the
  blocking narrator call returns (was cleared before, hiding it).
- **P2/P3** Hints probe stdout (not stdin); bloodline inherits the first
  inheritable talent among all picks instead of only pick #1 and no longer
  wipes an existing entry; dead `age <= 55` condition removed; stale
  comments updated.
- New regression gates: fact-reachability audit over all shards,
  context-key survival, chunk-boundary loop replay tests.

### Added
- scripts/test_pty.py: 8-case PTY end-to-end suite (full flow, step
  counting, q-quit, Ctrl+C cancel, pipe mode, seed determinism, CJK input,
  rapid paste) — 8/8 green.

## [0.6.0] - 2026-08-24

### Added
- Belief system expansion with three storylines driven by the fact engine:
  - **Faith arc** (`faith`): temple visits, small donations, volunteering,
    refuge — steady small comfort buffs; incense-economy satire.
  - **Superstition arc** (`superstition`): street fortune-telling ->
    pay-to-upgrade dependency -> "blood disaster" extortion -> info-gap
    reveal or broke awakening; dark mirror path as the scamming teller.
  - **Pseudo-science arc**: qigong cancer-cure masters and quantum speed-
    reading classes, both ending in exposure (based on real prosecuted
    cases).
- Two new careers: 算命先生/大师 (fortune teller) and 出家人 (monk, with
  tonsure/morning-bell/family-visit/return-to-world chain).
- 45 new events across `events_08_faith.json` and `events_09_streets2.json`,
  adapted from documented stories of Chinese documentary storytellers:
  Sanhe geopolitics seminars, the 15-yuan bunk, viral-fame arc (chosen ->
  crowded -> backlash -> gone), Myawaddy scam-park trap/escape, gambling
  spiral (first win -> KFC leftovers -> rehab day 30), massage-parlor scam,
  blind masseur, sugar-boss crash, and the "understands heaven and earth"
  sage NPC bridging the belief arcs.

## [0.5.0] - 2026-08-24

### Added
- Step-by-step advance mode: on interactive terminals each year waits for
  Enter (`q` quits mid-life without recording the bloodline; `--auto` or
  pipes stay continuous; force with `--step`).
- Storyline fact system: events can require/conflict/establish facts, so
  cult-childhood storylines stay coherent — a loving-home event never
  fires in a cult household, ordinary social life is locked while inside
  the sect and reopens after the rescue events (`sets: "!cult"`).
- Lifetime event uniqueness: no event repeats within one life.
- Talent overhaul: 59 talents across four rarity tiers (common / rare /
  epic / legendary) drawn with rarity weights and one guaranteed rare+
  per 10-pull.
- Rare surprise events shard (`events_07_rare.json`): lottery jackpot,
  talent-scout discovery, lightning survival, misdiagnosis reprieve and
  more.

### Fixed
- Custom stat allocation rejected zeros ("10 0 10 0"); zeros are legal
  again — the runtime init floor already prevents instant-death builds.
- Non-TTY input swallowed queued stdin lines (shared bufio.Scanner).
- Step mode felt dead with the LLM layer on: each narrated year blocked on
  the model call (up to 12 s of silence). A transient "命运编织中" hint now
  shows while the narrator works; PTY-verified that one Enter advances
  exactly one year.

# Changelog

All notable changes to this project are documented in this file.
Format follows Keep a Changelog; versioning follows SemVer.

## [0.4.0] - 2026-08-24

### Added
- Cult-family birth background (inherited stress sensitivity +0.25).
- 20 second-generation cult childhood/adulthood events
  (`events_06_cult_children.json`): confession assemblies, arranged
  betrothal, faith healing, shunning, deprogramming, reunions.
- Distinct death verdicts: toddlers ("幼年夭折") and minors
  ("未成年早逝") no longer share the adult depression label.

### Fixed
- Generation counter double-increment skipped every even generation.
- Digital nomad's `quit_if_stat: "mny"` rule was silently dead.
- LLM output could inject terminal escape sequences (OSC); all model text
  is now stripped of C0/C1 control bytes.
- Modified arrow keys (Ctrl/Alt+arrows) no longer jump to Home.
- Escape sequences split across terminal read chunks parse correctly
  instead of leaking as literal text.
- UTF-8 runes split across read chunks are reassembled, not dropped.
- Menus are exitable: Ctrl+C / Ctrl+D propagate a cancel signal.
- Bytes after an embedded newline in a paste survive into the next line.
- Zero-stat character builds can no longer die at age 0.

## [0.3.0] - 2026-08-23

### Added
- Trauma-dynamics core: leaky-integrator memory trace, amygdala/prefrontal
  coupling, reconsolidation feedback, hysteresis latch (EnterAt 0.70 /
  ExitAt 0.35), sub-additive intergenerational inheritance.
- AR(1) autocorrelated fortune replacing i.i.d. event rolls.
- 5 career decision windows per life; 24 careers including digital nomad,
  NEET, sex worker, adult-film performer, hostess, gigolo, erotica writer,
  sex therapist, cult leader (requires trauma load >= 0.45).
- 12 birth backgrounds shifting starting stats and trauma baseline.
- Optional LLM narration layer (OpenRouter `stealth/ox-alpha`): narrative
  rewriting, schema-validated fate events, epitaphs; fail-soft with a
  per-run call budget.
- Street-story event shard inspired by Chinese documentary storytellers
  (~220 events total across 6 shards).

## [0.1.0] - 2026-08-23

- Initial playable prototype.
