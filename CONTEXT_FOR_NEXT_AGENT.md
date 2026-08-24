# CONTEXT_FOR_NEXT_AGENT.md

最后更新: 2026-08-24 22:30

## 项目当前状态

rebirth v0.8.0 —— Go 终端人生重开模拟器，**可玩、已部署、公开仓库、全部测试绿、真实 LLM 路径已验证（含熔断器实测）**。

- 二进制: `~/.local/bin/rebirth`（每次改动后重新构建部署）
- 仓库: https://github.com/xieguiawu/rebirth（public，master）
- 数据: 26 职业 / 13 出身 / 63 天赋（四档稀有度+保底）/ **336 事件（9 分片）**
- 存档: `~/.config/rebirth/bloodline.json`（已清除重开）；配置: `~/.config/rebirth/config.json`（可选，flags > config > 默认）

## 架构

```
main.go                 入口：flags、出生/天赋/属性点流程、步进模式接线
internal/game/
  trauma.go             创伤动力学：漏积分器 m + 杏仁核 a + 前额叶 p 耦合 ODE，
                        迟滞闩锁 (EnterAt=0.80/ExitAt=0.35)、亚加性跨代遗传 ψ=0.7；
                        Drive/EventScale/阈值全部入 TraumaParams（配置可覆盖）
  events.go             Event(requires/conflict/sets/context)/Cond/Talent(rarity)/
                        Bloodline；加权抽取(AR(1)运势调制)+终身去重+Facts 引擎；
                        LoadEvents 用 DisallowUnknownFields（键漂移启动即炸）
  career.go             Career 26 条（含 requires_trauma_min 门控的邪教教主）+
                        Birth 13 种（sensitivity_add 抬创伤基线）
  run.go                主循环：职业窗口年龄{16,19,23,27,32,38,45}、步进暂停
                        (cfg.Step/Pause/Hints)、死亡判定与年龄分层标签
internal/llm/llm.go     provider 预设（deepseek 默认/openrouter）+ 每局预算（默认 24，墓志铭免预算）+
                        分级超时(12/18s) + **熔断器（连续 3 败→本世秒回本地，Broken() 接口）** +
                        JSON schema 校验 + clamp + stripControl + 纯文本兜底
internal/config/config.go 可选配置文件 ~/.config/rebirth/config.json：provider/model/llm_url/llm_calls/
                        narrate_ratio/max_age/seed/step/hints + trauma 动力学覆盖（enter_at/exit_at/drive/
                        event_trauma_scale）；未知键 DisallowUnknownFields 大声失败；坏文件 WARN 回退默认
internal/tui/input.go   零依赖 rune 安全行编辑器：feed() 纯函数解析，
                        rawCarry/carrySkipNL 跨调用状态，ErrCancelled 传播
scripts/test_pty.py     PTY 端到端套件（9 用例，stdlib only）
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
- v0.7.3 平衡实现从数据层换为代码层（EventTraumaScale=0.5，数据全部还原，500 局实测不变
  28.2%）；LLM 层 provider 化（openrouter/deepseek 预设 + --llm-url 覆盖 + deepseek-v4-flash）
- v0.7.4 配置文件化（internal/config，flags>config>默认）；trauma 动力学全部可调（Drive/EventScale
  入 TraumaParams）；LLM 预算重构（10→24、墓志铭免预算、narrate 按事件 ID 确定性采样 0.5）；
  **DeepSeek 真实 API 全链路首次验证成功**（fate+narrate+epitaph）
- v0.8.0 「命运编织中卡死」修复：根因 = OpenRouter 账户 402 无额度（付费模型全拒）+
  唯一可用的免费 stealth/ox-alpha 实测延迟 15.2s（叙事）/39.5s（命运）> 游戏 12/18s 超时 →
  每次调用烧满超时后必然失败且无熔断，每采一个事件冻 ~12s 一路爬行。修复：①Narrator 熔断器
  （连续 3 败→本世余下零网络秒回退，成功即重置；Run 打一次性提示）②默认 provider 切 deepseek
  （5.2s 实测、国内直连、免代理）③complete() 弃用无超时的 http.DefaultClient 兑底。实测：死服务器
  下 v0.7.4 90s 只到 12 岁 vs v0.8.0 36s 跑完 45 岁全程；DeepSeek 真机 45 岁全链路无熔断误杀

## 两轮 agent 审查的沉淀教训

1. struct tag/switch 分支与数据 key 的映射漂移已复发 4 次 → DisallowUnknownFields +
   TestFactReachability 双闸门已建，勿拆。
2. 单测手工拼接 feed 尾部会掩盖循环体 bug——循环级测试（driveLoop）必须保留。
3. ox-alpha 上游延迟可达 60s：LLM 层有预算+短超时+熔断器+纯文本兜底四层防御，勿移除。
   deepseek-v4-flash 偶发把 max_tokens 烧在 reasoning 上返回空 content（已知 flaky，兜底即安全）。

## 待办

- [ ] 交互模式完整人工测试（目前仅 PTY 自动化覆盖；v0.8.0 后建议玩家真机再过一遍）
- [x] 「命运编织中」卡死 ——v0.8.0 熔断器 + 默认渠道切换修复（根因：402 无额度 + ox-alpha 延迟超超时）
- [x] 病理态占比偏高（~60% 局）——v0.7.2 已标定至 28%（TestPathologicalRateBand 闸门 <50%，防复发）
- [x] LLM FateEvent 真实 API 成功路径——v0.7.4 用 DEEPSEEK_API_KEY 实测通过（墓志铭/命运/润色均真实生成）
- [x] 安卓移植架构方案——docs/plans/2026-08-24-rebirth-android-port-plan.md（ultrabrain：方案 C Go进程+JSON协议 推荐，7~10 人日）
- [ ] 属性点交互较简陋（一行四数字）；PTY 行编辑键位 e2e 已补（case I），全键位矩阵仍可加深
- [ ] graphify-out 已 gitignore，本地图谱需 `graphify update .` 手动重建

## 最后更新时间

2026-08-24 15:30
