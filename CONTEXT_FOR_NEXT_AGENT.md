# CONTEXT_FOR_NEXT_AGENT.md

最后更新: 2026-08-23 23:30

## 项目当前状态

rebirth v0.4.0 —— Go 终端人生重开模拟器，**可玩、已部署、全部测试通过**。

二进制: `~/.local/bin/rebirth`（每次改动后需重新构建部署）。

## v0.4.0 变更（momus 审查轮，3×P1 + 5×P2 + 3×P3 全部修复）

- **P1 代数双递增**：LoadBloodline 不再 ++，main 统一 `curGen = stored+1`（回归：连续两局显示第 1 代→第 2 代）
- **P1 数字游民 mny 退出条件失效**：careerQuitCheck 补 case "mny"（与历史 Cond-tag 同类 bug）
- **P1 终端转义注入**：新增 stripControl() 剥离 C0/C1 控制字节，覆盖 Narrate/Epitaph/FateEvent 全部输出路径
- **P2**：初始属性下限 1（孤儿流不再 0 岁夭折）、≤5 岁死亡标「幼年夭折」、属性点校验每项>0、修改箭头键（Ctrl+←→）不再误触 Home、ESC 序列跨 read 块拼接、UTF-8 rune 跨块重组、Ctrl+C/Ctrl+D 显式取消（ErrCancelled 传播出 Choose）、粘贴跨行字节保留到下次 ReadLine（rawCarry）
- **P3**：PickCareer 舍入兜底指向真实候选、draw[:3] 防越界
- **架构**：input.go 重写为 feed() 纯函数解析（可单测），ReadLineErr 返回 (string, error)
- 新数据：邪教家庭出身（sensitivity_add 0.25）+ events_06_cult_children.json 20 条第二代邪教成员童年/成年事件

## 架构

```
main.go                 入口：flag 解析、出生/天赋/属性点选择流程
internal/game/
  trauma.go             §XV 创伤动力学：漏积分器 m、杏仁核 a、前额叶 p，
                        迟滞双阈值 (EnterAt=0.70 / ExitAt=0.35)、亚加性遗传
  events.go             Stats/Event/Cond/Talent/Bloodline + 加权事件抽取
                        (AR(1) 运势调制) + 多分片 glob 加载 (data/events_*.json)
  career.go             Career 轨迹（24 条，含邪教教主 requires_trauma_min 门控）
                        + Birth 出身（12 种，sensitivity_add 抬高创伤基线）
  run.go                主循环：职业窗口年龄 {16,19,23,27,32,38,45}、
                        病理态 SPR 每年 -0.25、死亡判定 (STR<=0 或 SPR 低 5 年)
internal/llm/llm.go     OpenRouter 客户端 + Narrator（预算 DefaultCallBudget=10/局，
                        Narrate 12s / Fate 18s / Epitaph 12s 超时，
                        JSON schema 校验 + clamp + sanitizeLine 纯文本兜底）
internal/tui/input.go   零依赖 rune 安全行编辑器（raw mode + 非 TTY 降级）
```

## 数据集（5 个事件分片 ~220 条）

- `events_01_core.json` 人生主线 + 童年疗愈事件（防童年死亡螺旋）
- `events_02_love_sex.json` 情感与性（成人向，克制笔法：出柜、kink、STI、创伤后亲密等）
- `events_03_special.json` 特殊职业事件链（明星/男模/性工作者/成人影片/陪酒/舞男/CEO/科学家/邪教/电竞）
- `events_04_nomad_neet.json` 数字游民/NEET/情色小说家/性治疗师
- `events_05_streets.json` 街头故事致敬分片：三和大神/工地大猛子/患癌保安/杀马特原型 +
  摩的司机深夜乘客故事（被骗相亲女、出走大妈、玄学大学生、老刘）+ 投稿式情感故事 + 被博主采访的 meta 事件

## 关键设计决策

1. **随机性不交给 LLM**——确定性核心持有 RNG；LLM 只做叙事润色和命运事件注入（输出 clamp 到 ±3 / trauma_alpha≤0.5），失败即回退。
2. **病理态不是死刑**——2026-08-23 标定：Shock 再巩固增益 ×0.25、EnterAt 0.65→0.70、消退学习唤醒门 0.35→0.55、治疗在暴露期仍部分生效。10 种子分布：4 局安详离世。
3. **LLM 延迟防御**——ox-alpha 上游实测可达 60s+，故有每局 10 次调用预算 + 短超时 + 纯文本兜底。

## 已知问题 / 待办

- [ ] 交互模式未做完整人工测试（只测了 --auto 与非 TTY 降级）
- [ ] 属性点分配交互较简陋（一行四数字）
- [ ] 病理态占比仍偏高（6/10 局），可继续调 EnterAt 或事件 alpha 分布
- [ ] 无 interactive-cli-design §4 PTY 全键位测试套件（编辑器函数级已测）
- [ ] LLM FateEvent 从未在真实 API 下命中成功过（上游慢），schema 路径仅 mock 测试覆盖

## 血统存档位置

`~/.config/rebirth/bloodline.json`（Generation + Sensitivity + InheritedTal）

## 最后更新时间

2026-08-23 22:40
