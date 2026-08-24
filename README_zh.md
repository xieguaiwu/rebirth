**English** | [**中文版**](README.md)

# rebirth — 终端人生重开模拟器

Go 编写的终端人生重开模拟器。玩法致敬人生重开模拟器，内核换成计算精神病学取向的创伤记忆动力学模型。

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

## 它与同类游戏的区别

- **创伤是动力系统，不是 debuff 清单。** 漏积分器记忆痕迹与杏仁核反应性、前额叶控制耦合。持续逆境会把系统推过鞍结分岔、进入病理吸引子——而迟滞意味着：脱离的阈值比陷入时更低。治疗抵抗性，被数学化地模拟了出来。
- **命运是自相关的。** AR(1) 运势过程取代独立抽样：好运成串，灾祸也成串，逆袭需要持续的好年景而非一次暴击。
- **创伤跨代回响。** 每代继承上一代应激敏感性的亚加性比例（ψ=0.7）。血脉慢慢痊愈或螺旋下沉——你玩的每一局都在书写家族的基线。
- **大模型不掷骰子。** 全部随机性留在确定性核心。可选的模型层（默认 DeepSeek `deepseek-v4-flash`，也支持 OpenRouter）只做三件事：润色叙事、提出经 schema 校验的「命运事件」（数值强制 clamp）、撰写墓志铭。所有输出在接触你的终端前都会剥离控制字符。内置熔断器：连续 3 次失败（渠道死/额度尽/慢于 12/18s 超时）后本世余下瞬间切换纯本地叙事，不再反复白等。
- **内容广度**：63 天赋（四档稀有度）、26 职业（农民 → CEO → 数字游民 → 邪教教主* → 出家人）、13 出身（贫民窟到豪门到战乱地区）、9 分片 339 事件，含致敬国内纪录片式故事博主的街头故事。

\* 邪教教主路线仅在创伤负荷 ≥ 0.45 时解锁：被深渊浸透过的人，才开得了这场布道。

> **内容提示**：全篇成熟题材——创伤、精神疾病、性工作、性少数、邪教虐待。均为虚构，面向成年玩家；无露骨描写。

## 安装

```bash
go install github.com/xieguaiwu/rebirth@latest
# 或从源码：
git clone https://github.com/xieguaiwu/rebirth && cd rebirth
go build -o ~/.local/bin/rebirth .
```

需要 Go 1.25+。不做任何配置也可完全离线游玩。

## 使用

```bash
rebirth                          # 交互模式
rebirth --seed 42 --auto --no-llm   # 确定性自动模式（适合 CI）
```

| 参数 | 作用 |
|---|---|
| `--seed N` | 固定种子，完全可复现（配合 `--no-llm` 使用；可选的 LLM 层天然非确定性） |
| `--auto` | 自动选择所有选项 |
| `--no-llm` | 关闭 LLM 层 |
| `--step` | 强制逐条推进（每条等回车；交互终端默认开启） |
| `--provider NAME` | LLM 服务商预设：`deepseek`（默认，国内直连免代理）或 `openrouter`（需代理+额度） |
| `--model NAME` | 模型名（按 provider 默认：`deepseek-v4-flash` / `stealth/ox-alpha`） |
| `--llm-url URL` | 覆盖 provider 基础 URL（任意 OpenAI 兼容端点） |

设置 `DEEPSEEK_API_KEY`（默认服务商）、`OPENROUTER_API_KEY`（配合
`--provider openrouter`）或通用 `LLM_API_KEY` 启用叙事层。反复失败的渠道会触发熔断——打印一行提示后，本世余下全部秒回本地文本。血统存档位于
`~/.config/rebirth/bloodline.json`；删除它即可开启新家族。

### 可选配置文件

`~/.config/rebirth/config.json` 持久化你的默认设置——优先级：命令行参数
> 配置文件 > 内置默认。所有字段均可选：

```json
{
  "provider": "deepseek",
  "model": "",
  "llm_url": "",
  "llm_calls": 24,
  "narrate_ratio": 0.5,
  "max_age": 100,
  "seed": 0,
  "step": false,
  "hints": true,
  "trauma": {
    "enter_at": 0.80,
    "exit_at": 0.35,
    "drive": 0.65,
    "event_trauma_scale": 0.5
  }
}
```

- `llm_calls`：每局 LLM 调用预算（叙事润色+命运事件）。墓志铭不计入，永不被饿死。
- `narrate_ratio`：创伤/正面事件被 LLM 润色的比例（0.5=一半，按事件 ID 确定性采样）。
- `trauma.*`：病理吸引子动力学参数覆盖——v0.7.2 标定的默认值（病理率 ~28%）已列出；
  想更少创伤可调高 `enter_at`，想更温和的人生可调低 `drive`。
- 未知键会启动时 WARN 拒绝（拼写错误大声失败，不静默）。

## 项目结构

```
main.go                 入口：参数解析、出生/天赋/属性点流程
internal/game/
  trauma.go             耦合 ODE 核心 + 迟滞 + 遗传核
  events.go             加权事件抽取、天赋、血统治存档
  career.go             职业轨迹 + 出身背景
  run.go                主循环、死亡判定、叙事钩子
internal/llm/           provider 预设（deepseek/openrouter），带预算+熔断的 fail-soft 叙事器
internal/tui/           零依赖 rune 安全行编辑器
internal/game/data/     events_*.json × 9 分片、职业、出身、天赋
```

## 开发

```bash
go vet ./... && go test ./...
go build -o ~/.local/bin/rebirth .   # 改动后部署到本地
```

测试覆盖：迟滞不对称性、冲击有界性、亚加性遗传、种子确定性、职业门控（含创伤门槛的邪教路线）、CJK 安全编辑、输入跨块解析、LLM 回退路径。

## 许可

[MIT](LICENSE)
