# CONTEXT_FOR_NEXT_AGENT.md

最后更新: 2026-08-24 14:25

## 项目当前状态

rebirth v0.7.2 —— Go 终端人生重开模拟器，**可玩、已部署、公开仓库、全部测试绿**。

- 二进制: `~/.local/bin/rebirth`（每次改动后重新构建部署）
- 仓库: https://github.com/xieguiawu/rebirth（public，master）
- 数据: 26 职业 / 13 出身 / 63 天赋（四档稀有度+保底）/ **336 事件（9 分片）**

## 架构

```
main.go                 入口：flags、出生/天赋/属性点流程、步进模式接线
internal/game/
  trauma.go             创伤动力学：漏积分器 m + 杏仁核 a + 前额叶 p 耦合 ODE，
                        迟滞闩锁 (EnterAt=0.70/ExitAt=0.35)、亚加性跨代遗传 ψ=0.7
  events.go             Event(requires/conflict/sets/context)/Cond/Talent(rarity)/
                        Bloodline；加权抽取(AR(1)运势调制)+终身去重+Facts 引擎；
                        LoadEvents 用 DisallowUnknownFields（键漂移启动即炸）
  career.go             Career 26 条（含 requires_trauma_min 门控的邪教教主）+
                        Birth 13 种（sensitivity_add 抬创伤基线）
  run.go                主循环：职业窗口年龄{16,19,23,27,32,38,45}、步进暂停
                        (cfg.Step/Pause/Hints)、死亡判定与年龄分层标签
internal/llm/llm.go     OpenRouter 客户端：每局预算 10 次、分级超时(12/18s)、
                        JSON schema 校验+clamp+stripControl(C0/C1 剥离)+
                        sanitizeLine 纯文本兜底
internal/tui/input.go   零依赖 rune 安全行编辑器：feed() 纯函数解析，
                        rawCarry/carrySkipNL 跨调用状态，ErrCancelled 传播
scripts/test_pty.py     PTY 端到端套件（8 用例，stdlib only）
```

## 事实引擎（连贯性核心）

事件三字段：`requires`(事实必须为真) / `conflict`(事实必须为假) / `sets` 或
`context`(触发时建立事实，`!fact` 语法清除)。出身经 NewFacts 预设（cult_family→"cult" 等）。
弧线：cult 邪教（含第二代童年分片）、faith 正信、superstition 迷信（黑暗镜像 scammer）、
famous 流量、gambler 赌球、park_risk 妙瓦底。TestFactReachability 保证所有引用的事实可产出。

## 版本史要点

- v0.5.0 步进模式 / 天赋稀有度 / 终身事件唯一性
- v0.6.0 信仰三弧线 + 街头故事 II（researcher agent 调研驱动）
- v0.7.0 momus round-2 的 9 findings 全修（血统腐蚀/context 键漂移/feed 循环丢字节）
- v0.7.1 oracle 复核补充修复（InheritedTal 播种、orphan \n 幻影提交、carried CSI 吞 Ctrl+C）
- v0.7.2 病理态平衡标定（83%→28%：事件 alpha 减半 + drive 0.65 + EnterAt 0.80，
  TestPathologicalRateBand 回归闸门）；LowStreak 死状态移除；PTY 套件 9 用例

## 两轮 agent 审查的沉淀教训

1. struct tag/switch 分支与数据 key 的映射漂移已复发 4 次 → DisallowUnknownFields +
   TestFactReachability 双闸门已建，勿拆。
2. 单测手工拼接 feed 尾部会掩盖循环体 bug——循环级测试（driveLoop）必须保留。
3. ox-alpha 上游延迟可达 60s：LLM 层有预算+短超时+纯文本兜底三层防御，勿移除。

## 待办

- [ ] 交互模式完整人工测试（目前仅 PTY 自动化覆盖）
- [x] 病理态占比偏高（~60% 局）——v0.7.2 已标定至 28%（TestPathologicalRateBand 闸门 <50%，防复发）
- [ ] LLM FateEvent 真实 API 成功路径未验证过（仅 mock 覆盖）
- [ ] 属性点交互较简陋（一行四数字）；PTY 行编辑键位 e2e 已补（case I），全键位矩阵仍可加深
- [ ] graphify-out 已 gitignore，本地图谱需 `graphify update .` 手动重建

## 最后更新时间

2026-08-24 14:25
