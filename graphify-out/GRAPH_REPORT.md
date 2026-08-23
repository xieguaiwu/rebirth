# Graph Report - rebirth  (2026-08-24)

## Corpus Check
- 23 files · ~10,156 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 177 nodes · 397 edges · 11 communities (10 shown, 1 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 59 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `bb58b383`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- TraumaParams
- Run
- input.go
- llm.go
- game_test.go
- main
- rebirth — Terminal Life Restart Simulator
- CONTEXT_FOR_NEXT_AGENT.md
- NewNarrator
- input_test.go
- rebirth

## God Nodes (most connected - your core abstractions)
1. `Run()` - 28 edges
2. `feed()` - 17 edges
3. `main()` - 15 edges
4. `LineEditor` - 12 edges
5. `Stats` - 11 edges
6. `Talent` - 10 edges
7. `Career` - 9 edges
8. `Event` - 9 edges
9. `NewNarrator()` - 9 edges
10. `TestFullAutoRun()` - 8 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `LoadEvents()`  [INFERRED]
  main.go → internal/game/events.go
- `main()` --calls--> `LoadTalents()`  [INFERRED]
  main.go → internal/game/events.go
- `pickTalents()` --calls--> `DrawTalents()`  [INFERRED]
  main.go → internal/game/events.go
- `main()` --calls--> `LoadBloodline()`  [INFERRED]
  main.go → internal/game/events.go
- `main()` --calls--> `Run()`  [INFERRED]
  main.go → internal/game/run.go

## Import Cycles
- None detected.

## Communities (11 total, 1 thin omitted)

### Community 0 - "TraumaParams"
Cohesion: 0.44
Nodes (8): TraumaParams, TraumaState, stateSummary(), clamp(), clamp01(), EndingSensitivity(), InheritSensitivity(), NewTraumaState()

### Community 1 - "Run"
Cohesion: 0.10
Nodes (31): Bloodline, Career, Cond, Config, Effects, Event, Fortune, Narrator (+23 more)

### Community 2 - "input.go"
Cohesion: 0.18
Nodes (16): applyFinalByte(), cloneBytes(), dispatchCSI(), feed(), ioctl(), IsTTY(), leadingDigits(), makeRaw() (+8 more)

### Community 3 - "llm.go"
Cohesion: 0.25
Nodes (12): Context, extractJSON(), New(), sanitizeLine(), stripControl(), truncate(), chatMessage, chatRequest (+4 more)

### Community 4 - "game_test.go"
Cohesion: 0.24
Nodes (13): discard, LoadEvents(), T, TestApplyDeltaClamps(), TestCareerGateOnEvents(), TestCultLeaderRequiresTrauma(), TestExtinctionRequiresSafeContext(), TestHysteresisAsymmetry() (+5 more)

### Community 5 - "main"
Cohesion: 0.24
Nodes (15): Birth, DrawBirths(), LoadBirths(), LoadCareers(), TestFullAutoRun(), Choose(), allocatePoints(), buildNarrator() (+7 more)

### Community 6 - "rebirth — Terminal Life Restart Simulator"
Cohesion: 0.17
Nodes (10): Highlights, Install & Play, License, rebirth — Terminal Life Restart Simulator, Testing, rebirth — 终端人生重开模拟器, 安装与游玩, 测试 (+2 more)

### Community 7 - "CONTEXT_FOR_NEXT_AGENT.md"
Cohesion: 0.20
Nodes (8): v0.4.0 变更（momus 审查轮，3×P1 + 5×P2 + 3×P3 全部修复）, 关键设计决策, 已知问题 / 待办, 数据集（5 个事件分片 ~220 条）, 最后更新时间, 架构, 血统存档位置, 项目当前状态

### Community 8 - "NewNarrator"
Cohesion: 0.44
Nodes (10): NewNarrator(), T, mockServer(), TestEpitaphFallback(), TestFateEventInvalidRejected(), TestFateEventTextStripped(), TestFateEventValidJSONAccepted(), TestNarrateFallsBackOnError() (+2 more)

### Community 9 - "input_test.go"
Cohesion: 0.26
Nodes (13): T, TestBackwardCJKSafe(), TestFeedCtrlDCancels(), TestFeedDeleteKey(), TestFeedModifiedArrowNotHome(), TestFeedSplitEscape(), TestFeedSplitRuneCarried(), TestFeedSubmitCarriesRemainder() (+5 more)

## Knowledge Gaps
- **19 isolated node(s):** `rebirth`, `chatResponse`, `fatePayload`, `项目当前状态`, `v0.4.0 变更（momus 审查轮，3×P1 + 5×P2 + 3×P3 全部修复）` (+14 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `main` to `TraumaParams`, `Run`, `llm.go`, `game_test.go`?**
  _High betweenness centrality (0.249) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `TraumaParams`, `llm.go`, `game_test.go`, `main`?**
  _High betweenness centrality (0.242) - this node is a cross-community bridge._
- **Why does `Choose()` connect `main` to `input.go`?**
  _High betweenness centrality (0.208) - this node is a cross-community bridge._
- **Are the 11 inferred relationships involving `Run()` (e.g. with `TestFullAutoRun()` and `PickCareer()`) actually correct?**
  _`Run()` has 11 INFERRED edges - model-reasoned connections that need verification._
- **Are the 6 inferred relationships involving `feed()` (e.g. with `TestFeedCtrlDCancels()` and `TestFeedDeleteKey()`) actually correct?**
  _`feed()` has 6 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `main()` (e.g. with `LoadBirths()` and `LoadCareers()`) actually correct?**
  _`main()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `rebirth`, `chatResponse`, `fatePayload` to the rest of the system?**
  _19 weakly-connected nodes found - possible documentation gaps or missing edges._