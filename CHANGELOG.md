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
