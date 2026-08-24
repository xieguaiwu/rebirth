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
