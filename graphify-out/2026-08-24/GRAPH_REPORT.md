# Graph Report - rebirth  (2026-08-23)

## Corpus Check
- 22 files · ~9,041 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 162 nodes · 356 edges · 11 communities (10 shown, 1 thin omitted)
- Extraction: 86% EXTRACTED · 14% INFERRED · 0% AMBIGUOUS · INFERRED: 51 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `651ecb18`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Run
- events.go
- ReadLine
- llm.go
- game_test.go
- main
- rebirth — Terminal Life Restart Simulator
- CONTEXT_FOR_NEXT_AGENT.md
- NewNarrator
- input_test.go
- rebirth

## God Nodes (most connected - your core abstractions)
1. `Run()` - 27 edges
2. `ReadLine()` - 16 edges
3. `main()` - 15 edges
4. `Stats` - 11 edges
5. `Talent` - 10 edges
6. `LineEditor` - 10 edges
7. `Career` - 9 edges
8. `Event` - 9 edges
9. `TestFullAutoRun()` - 8 edges
10. `TraumaParams` - 8 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `LoadCareers()`  [INFERRED]
  main.go → internal/game/career.go
- `main()` --calls--> `LoadBirths()`  [INFERRED]
  main.go → internal/game/career.go
- `pickBirth()` --calls--> `DrawBirths()`  [INFERRED]
  main.go → internal/game/career.go
- `main()` --calls--> `LoadEvents()`  [INFERRED]
  main.go → internal/game/events.go
- `main()` --calls--> `LoadTalents()`  [INFERRED]
  main.go → internal/game/events.go

## Import Cycles
- None detected.

## Communities (11 total, 1 thin omitted)

### Community 0 - "Run"
Cohesion: 0.17
Nodes (19): Config, Narrator, noop, Talent, TraumaParams, TraumaState, LoadTalents(), isNoop() (+11 more)

### Community 1 - "events.go"
Cohesion: 0.15
Nodes (20): Birth, Career, Cond, Effects, Event, Fortune, Result, Stats (+12 more)

### Community 2 - "ReadLine"
Cohesion: 0.21
Nodes (12): decodeRune(), handleCSI(), handleSS3(), ioctl(), IsTTY(), makeRaw(), Pause(), ReadLine() (+4 more)

### Community 3 - "llm.go"
Cohesion: 0.26
Nodes (11): Context, extractJSON(), New(), sanitizeLine(), truncate(), chatMessage, chatRequest, chatResponse (+3 more)

### Community 4 - "game_test.go"
Cohesion: 0.23
Nodes (14): discard, LoadCareers(), LoadEvents(), T, TestApplyDeltaClamps(), TestCareerGateOnEvents(), TestCultLeaderRequiresTrauma(), TestExtinctionRequiresSafeContext() (+6 more)

### Community 5 - "main"
Cohesion: 0.25
Nodes (12): Bloodline, LoadBloodline(), Choose(), allocatePoints(), buildNarrator(), Rand, inheritTalent(), main() (+4 more)

### Community 6 - "rebirth — Terminal Life Restart Simulator"
Cohesion: 0.17
Nodes (10): Highlights, Install & Play, License, rebirth — Terminal Life Restart Simulator, Testing, rebirth — 终端人生重开模拟器, 安装与游玩, 测试 (+2 more)

### Community 7 - "CONTEXT_FOR_NEXT_AGENT.md"
Cohesion: 0.22
Nodes (7): 关键设计决策, 已知问题 / 待办, 数据集（5 个事件分片 ~220 条）, 最后更新时间, 架构, 血统存档位置, 项目当前状态

### Community 8 - "NewNarrator"
Cohesion: 0.50
Nodes (8): NewNarrator(), T, mockServer(), TestEpitaphFallback(), TestFateEventInvalidRejected(), TestFateEventValidJSONAccepted(), TestNarrateFallsBackOnError(), Server

### Community 9 - "input_test.go"
Cohesion: 0.39
Nodes (8): T, TestBackwardCJKSafe(), TestDecodeRuneSplitBytes(), TestForwardDelete(), TestInsertAdvancesByRune(), TestKillBeforeAfter(), TestKillWordBackMixed(), TestSkipToTerminator()

## Knowledge Gaps
- **18 isolated node(s):** `rebirth`, `chatResponse`, `fatePayload`, `项目当前状态`, `架构` (+13 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `main` to `Run`, `events.go`, `llm.go`, `game_test.go`?**
  _High betweenness centrality (0.266) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `events.go`, `llm.go`, `game_test.go`, `main`?**
  _High betweenness centrality (0.265) - this node is a cross-community bridge._
- **Why does `ReadLine()` connect `ReadLine` to `main`?**
  _High betweenness centrality (0.204) - this node is a cross-community bridge._
- **Are the 11 inferred relationships involving `Run()` (e.g. with `TestFullAutoRun()` and `PickCareer()`) actually correct?**
  _`Run()` has 11 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `main()` (e.g. with `LoadBirths()` and `LoadCareers()`) actually correct?**
  _`main()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `rebirth`, `chatResponse`, `fatePayload` to the rest of the system?**
  _18 weakly-connected nodes found - possible documentation gaps or missing edges._