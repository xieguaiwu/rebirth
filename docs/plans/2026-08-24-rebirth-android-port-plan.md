# rebirth 安卓移植 + F-Droid 发布架构方案

> 日期：2026-08-24 · 依据代码核验（commit d5a85b5，v0.7.4）
> 结论先行：**方案 C（Go 核心编译为安卓可执行文件 + stdio JSON 协议 + Kotlin/Compose UI）为推荐主方案**；
> gomobile bind（方案 A）为备选；纯 Kotlin 重写（方案 B）否决。
> 所有标注 ⚠️需验证 的条目为未实测假设，禁止当作既定事实。

---

## 0. TL;DR 结论摘要

| 决策点 | 结论 |
|---|---|
| 移植策略 | **C：Go 进程 + JSON-lines 协议**。零 NDK、零 cgo、纯 Go 交叉编译，F-Droid 可复现性最高，代码 100% 复用 |
| 备选 | A：gomobile bind（若真机 exec 受挫）；A/C 共享同一会话化重构，切换成本低 |
| 数据带入 | `go:embed` 编入 Go 二进制，安卓侧零数据搬运、零漂移 |
| 存档位置 | `filesDir/rebirth/bloodline.json`，与桌面端同 schema，跨端互通 |
| LLM | Go 侧 `internal/llm` 原样复用；key 存 Android Keystore，由 Kotlin 注入进程内存 |
| 确定性 | `math/rand` 旧版序列被 Go 1 兼容承诺冻结 + 纯 Go 整数实现 + IEEE 754 浮点 → 同种子跨 OS/架构复现（设备端 golden 测试锁定） |
| F-Droid | 声明 `NonFreeNet`（LLM 叙事可选）；prebuild 下载固定版本 Go 工具链；双构建 SHA-256 验证 |
| 估时 | 内部 7~10 人日（P0 4~6 + P1 2~3 + P2 1~2）+ F-Droid 审核排队 1~4 周（外部） |

---

## 1. 现状核验（2026-08-24 实测，勿再凭记忆假设）

| 文件 | 核验事实 | 移植影响 |
|---|---|---|
| `go.mod` | `module rebirth`，`go 1.25`，**零第三方依赖**；本机 go1.25.10 linux/amd64 | F-Droid 构建只需 Go 工具链 |
| `internal/game/run.go` | 唯一输出通道 `io.Writer w`；imports 仅 `fmt, hash/fnv, io, math, math/rand, strings`——**无 os/syscall/终端**。ANSI 提示串 `hintPending` 仅在 `cfg.Hints` 时写入（main 按 TTY 设置），移动端置 false 即干净 | 核心可直接编译进安卓，无需改动 |
| `internal/game/events.go` | `//go:embed data/*.json`；`os` 仅用于血统存档读写（路径由调用方传入）；`facts`/`used` 均为 map 只查不遍历 → 无 map 迭代序不确定性问题 | 数据随二进制走，零资源搬运 |
| `internal/game/trauma.go` | 纯 `math`，耦合 ODE + 迟滞双阈值（EnterAt=0.80/ExitAt=0.35） | 平台无关 |
| `internal/game/career.go` | 职业/出身表 + 加权抽取，纯标准库 | 平台无关 |
| `internal/game/data/` | 实测计数：**336 事件 / 26 职业 / 13 出身 / 63 天赋**，共 124KB JSON | embed 后二进制 +124KB，可忽略 |
| `internal/llm/llm.go` | `net/http` + `context` 超时；**key 由 `llm.New(key, model)` 注入**（main 从环境变量读）——包本身零平台依赖 | 安卓侧由 UI 注入 key 即可，包无需改动 |
| `internal/config/config.go` | `DefaultPath()` 用 `os.UserConfigDir()`（安卓 GOOS 下 HOME 通常为 `/`，**不可写**）——仅 main.go 调用 | 移动端绕过 `DefaultPath`，显式传路径 |
| `internal/tui/` | `syscall`/termios/`unsafe` 终端编辑器 | **安卓不可编译，整体替换**（预期内，仅交互输入用） |
| `main.go` | 菜单绘制/选择/属性分配在 main；**关键发现：`Run()` 内部 `rng := rand.New(rand.NewSource(cfg.Seed))` 创建全新 RNG，与 main 的抽取 RNG 互相独立**（抽取与人生是同一 seed 的两条独立随机流） | 协议上抽取命令可无状态化；重放恢复只需 cfg+选择 |
| `main.go` `pickBirth` 等 | 出生 3 选 1、天赋 10 选 3、属性 20 点分配 | UI 层用 Compose 重画，抽取逻辑仍在 Go |
| 测试 | `go test ./...` 全绿（game/llm/config/tui 四包） | 会话化重构后必须保持全绿 + 新增 golden 测试 |
| git | 公开仓库 github.com/xieguaiwu/rebirth，**零 tag**（F-Droid 发布前置项） | P2 打 tag |

**结论：任务描述中「internal/game 零终端依赖」属实**。`events.go` 虽 import `os`，但仅用于血统存档的 `ReadFile/MkdirAll/WriteFile`（路径参数化），无任何终端/环境依赖。`io.Writer` 抽象干净：全部输出经 `record()` → `fmt.Fprintln(w, ...)`。

---

## 2. 架构总览（推荐方案 C）

```
┌──────────────────── Android 应用 com.xieguiawu.rebirth ────────────────────┐
│  Kotlin / Jetpack Compose（UI 层 —— 唯一的安卓原生代码）                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────────┐ ┌───────────┐ ┌────────────┐ │
│  │ 族谱/主页 │ │ 角色创建  │ │ 人生时间线     │ │ 创伤面板   │ │ 设置/种子   │ │
│  └────┬─────┘ └────┬─────┘ └──────┬───────┘ └─────┬─────┘ └─────┬──────┘ │
│       └────────────┴──────────────┴───────────────┴──────────────┘        │
│                    AppState（ViewModel / 单 Activity）                     │
│   bloodline.json ─ filesDir/rebirth/           API key ─ Android Keystore  │
│   session.json  ─ filesDir/rebirth/           (AES-GCM, 仅出内存进 Go 进程) │
├──────────────────── 进程桥（Kotlin，~200 行，无 JNI 代码）──────────────────┤
│  ProcessBuilder exec(nativeLibraryDir/librebirth_core.so)                  │
│  stdin/stdout JSON-lines 协议 · Dispatchers.IO 异步读写 · 超时/崩溃重启     │
│  后台杀进程 → session.json + 重放恢复                                        │
├──────────────────── Go 核心进程 librebirth_core.so（GOOS=android,CGO=0）────┤
│  cmd/mobile：JSON-lines daemon（桌面可直接运行同二进制逻辑，便于 CI 测试）      │
│  ┌─────────────┐  ┌──────────────────┐  ┌─────────────┐  ┌──────────────┐ │
│  │ session.go  │→ │ internal/game    │  │ internal/llm │  │ internal/    │ │
│  │ 可恢复步进器 │  │ 创伤ODE/事件/职业   │  │ 叙事/命运事件 │  │ config(合并)  │ │
│  └─────────────┘  └──────────────────┘  └─────────────┘  └──────────────┘ │
│  go:embed data/*.json（数据随二进制，零资源搬运，零漂移）                      │
│  math/rand 确定性核心（同种子 = 同人生，与桌面 CLI 逐字一致）                   │
└────────────────────────────────────────────────────────────────────────────┘
        │ 仅 LLM 叙事时出网（HTTPS，可完全离线运行）
        ▼
  OpenRouter / DeepSeek（可选 → NonFreeNet 反特性声明）
```

### 2.1 JSON-lines 协议草案（进程桥）

每行一个 JSON 对象，`id` 关联请求/响应；响应 `{id, ok, data?|error?}`。

```json
→ {"id":1,"cmd":"hello"}                        ← {"id":1,"ok":true,"data":{"ver":"0.8.0"}}
→ {"id":2,"cmd":"draw_births","seed":12345}      ← {"id":2,"ok":true,"data":{"births":[{"name":"孤儿",...}×3]}}
→ {"id":3,"cmd":"draw_talents","seed":12345}     ← 10 张天赋（含保底稀有）
→ {"id":4,"cmd":"new_session","cfg":{"seed":12345,"birth":"孤儿","talents":["乐天派"],
     "points":[5,5,5,5],"max_age":100,"gen":3,"sens":0.31,"narrator":{"provider":"deepseek",
     "model":"deepseek-v4-flash","url":"","key":"<进程内存注入>","budget":24,"ratio":0.5}}}
→ {"id":5,"cmd":"next"}                          ← {"id":5,"ok":true,"data":{"year":{
     "age":7,"text":"[7岁] …","event":"bully_01","trauma_alpha":0.3,"therapy_q":0,
     "career":"无业","career_change":null,"stats":{"chr":5.2,"int":6.0,"str":4.8,"mny":3.1,"spr":7.4},
     "trauma":{"m":0.31,"a":0.42,"p":0.63,"load":0.35,"pathological":false},
     "luck":0.12,"llm":false,"died":false}}}
→ {"id":6,"cmd":"next"}                          ← …直到 died:true + "epitaph"
→ {"id":7,"cmd":"shutdown"}
```

要点：
- 抽取命令（`draw_births`/`draw_talents`）**无状态**——复用「同一 seed 两条独立随机流」的既有行为，与 CLI 逐位一致。
- `next` 阻塞上限 = LLM 超时（≤18s）；Kotlin 侧 await 带超时 + 「命运编织中」提示（对应现有 `Hints` 机制）。
- Go 侧 `bufio.Scanner(stdin)` + `json.Encoder(stdout)` 逐行刷新；stderr 走日志。
- daemon 逻辑平台无关 → 可在桌面直接跑 `cmd/mobile` 与 CLI 做 golden 对比（CI 友好）。

### 2.2 打包机制（exec 的落点）

- `go build` 产出 ET_DYN PIE 静态二进制（android 默认 buildmode=pie），改名为 `librebirth_core.so` 放入 `android/app/src/main/jniLibs/<abi>/`。
- Gradle 需 `packaging { jniLibs { useLegacyPackaging = true } }`（即 `extractNativeLibs=true`）→ 安装时 .so 被解压到 `nativeLibraryDir`（只读、系统属主，API 29+ 仍可 exec）。
- Kotlin 侧 `ProcessBuilder(context.applicationInfo.nativeLibraryDir + "/librebirth_core.so")` 启动子进程（继承 UID → INTERNET 权限适用）。
- ⚠️需验证：`exec()` 从 nativeLibraryDir 直接启动 ELF 在 targetSdk 35 真机行为（社区通行做法，模拟器类 app 常用，但必须在 Pixel 上实测）。若受挫 → 切方案 A。

---

## 3. 问题 1：移植策略对比（关键决策）

| 维度 | A. gomobile bind + Compose | B. 纯 Kotlin 重写 | **C. Go 进程 + JSON 协议（推荐）** |
|---|---|---|---|
| 代码复用率 | 高：game+llm 全复用，需 gobind 门面层 | 低：重写引擎 ~800 行 + 采样 ~300 行；JSON 数据可共享但引擎漂移风险大 | **最高：整个 Go 程序原样复用，仅新增入口 cmd/mobile** |
| 确定性/行为一致 | 与 CLI 完全一致（同一 Go 代码） | 需移植 Go `rngSource` 算法 + golden 测试，双引擎调参永远有漂移风险 | **与 CLI 完全一致（同一 Go 代码）** |
| 维护成本 | 中：gobind 类型限制（无函数类型/泛型、接口绑定受限）、gomobile 与 Go 版本锁步 | 高：平衡性改动要改两处 | **低：单引擎，协议稳定后几乎不动** |
| F-Droid 可复现 | 中：需 NDK + cgo + gobind 工具链，cgo 产物可复现性有历史风险 | 高（姊妹项目已验证路径） | **高：纯 Go 交叉编译（CGO_ENABLED=0），无 NDK；Gradle 侧沿用已验证的无 R8 配置** |
| 包体积 | ~2.5MB/ABI（aar） | 最小 | ~2.5MB/ABI（静态二进制，`-ldflags="-s -w"` 可再压 ~30%），AAB 按 ABI 分发，F-Droid 通用包含 4 ABI 约 10~11MB |
| 发布复杂度 | 中 | 低 | 中：exec 落点受 Android 安全策略约束（需真机验证，见 §2.2） |
| 调试体验 | JNI 原生崩溃难查（tombstone） | 原生工具链最好 | **进程边界清晰：崩溃=进程退出可重启，stderr 直读日志** |
| 步进交互 | 阻塞回调跨界（Java 实现 Go 接口）awkward | 天然 | **pull 式 `next` 天然匹配步进玩法** |

**推荐 C 的理由**：
1. 本游戏是**步进驱动**（一年一敲回车），进程间 JSON 往返延迟 <1ms，进程模型的唯一劣势（延迟）不存在；
2. 人生过程内**零玩家选择**（职业切换由 RNG 决定，Pause 仅退出）→ 一局 = 纯函数(seed, 出身, 天赋, 属性, 配置)，重放恢复方案天然成立（§5.4）；
3. F-Droid 可复现是姊妹项目花大力气验证过的最大痛点，方案 C 的构建链最短：一个固定版本 Go tarball + 零 NDK/零 cgo/零 R8；
4. 零依赖 ethos 的延续：不引入 gomobile/x/mobile 这条半维护状态的工具链；
5. 会话化重构（§5.3）后 A/C 共用同一 Session API，仅传输层不同——**即使 C 真机受阻，切 A 的沉没成本约 1 人日**，决策可逆。

**A 作为备选**（真机 exec 受挫时启用）：需 `gomobile init`（NDK）、gobind 门面层用受限类型重写协议、R8 keep 规则（`-keep class go.**`）、F-Droid prebuild 增加 gomobile 安装步骤。

**B 否决**：核心卖点（创伤动力学 + 加权抽取 + 事实引擎的精确平衡）双份实现意味着每次平衡调参（v0.7.x 的教训就是连续校准）都要同步两套引擎并验证一致性，长期成本不可接受。数据 JSON 的共享仅覆盖内容不覆盖行为。

---

## 4. 问题 2：核心复用边界（已核验）

### 4.1 io.Writer 抽象核验结论
- `game.Run(w io.Writer, cfg, evs, careers)` 为唯一入口，全部输出经 `w`；`cfg.Hints` 是唯一的终端耦合点（ANSI 串），移动端置 false 即可。
- `cfg.Pause func() bool` 是步进回调（main 注入终端输入）——**会话化后移动端不再需要 Pause**（UI 用 `next` 驱动），CLI 的 Run 保留 Pause 语义。
- `main.go` 的菜单/选择逻辑不属 core，移动端在 Compose 层重画，抽取结果由 Go 侧命令返回。

### 4.2 embed 数据带入安卓
- 方案 C/A 下 `go:embed data/*.json` 原样编译进 Go 二进制/aar——**零工作量、零漂移、单一内容源**。桌面与手机同一份数据。
- 若未来想热更新内容（F-Droid 应用每次更新要等 review，玩家可自建事件分片）：可选在 session 启动时扫描 `filesDir/rebirth/data/events_*.json` 并**追加**到 embed 分片之后（需保留 CLI 的 `DisallowUnknownFields` 校验）。非 MVP 必需。

### 4.3 存档与配置位置
| 数据 | 桌面 | Android | 兼容方案 |
|---|---|---|---|
| 血统 `bloodline.json` | `~/.config/rebirth/` | `context.filesDir/rebirth/bloodline.json` | **同 JSON schema**，跨端互通（手机玩一代、拷回桌面续族谱） |
| 配置 `config.json` | `~/.config/rebirth/config.json` | 设置页为主；`filesDir/rebirth/config.json` 兼容读（同 schema） | 设置页字段与桌面 config 字段一一对应 |
| 会话 `session.json`（新） | 无 | `filesDir/rebirth/session.json` | 每 `next` 后落盘，进程被杀可恢复（§5.4） |
| LLM key | 环境变量 | Android Keystore（AES-GCM，同姊妹项目 `api_checkers_master` 模式） | 仅由 Kotlin 读出注入 Go 进程内存，**Go 侧永不落盘** |

⚠️ 注意：`config.DefaultPath()` 在安卓 GOOS 下依赖 `os.UserConfigDir()`（HOME=/），**必须绕过**——cmd/mobile 接受显式路径参数，Kotlin 传入 `filesDir`。

### 4.4 会话化重构（所有方案的共同前置，~1.5 人日）
新增 `internal/game/session.go`，把 `Run` 的循环体拆为可恢复步进器：

```go
type Session struct{ /* cfg,rng,trauma,s,fortune,facts,used,age,careerID,history,… */ }
func NewSession(cfg Config, evs []Event, careers []*Career) *Session   // Run 的初始化段
func (s *Session) Advance() YearResult    // 一年：职业窗→事件抽取(含LLM)→trauma.Step→病态衰减
func (s *Session) DeathCheck() YearResult // SPR/STR/寿命判定（与 Advance 分两阶段）
```

- **两阶段拆分的原因**：现有 `Run` 的 Pause 发生在 `trauma.Step` 之后、死亡判定之前；拆两段才能让重构后的 `Run` 输出与现有 CLI 逐字节一致（golden 测试可断言），CLI 行为零变化。
- `Run` 重写为 Session 的薄循环（打印 + Pause），`game_test.go` 全部测试必须原样通过。
- 确定性重放 = `NewSession(同 cfg, LLM=Noop)` + 循环 `Advance` 到目标年龄——所有 RNG 抽取顺序相同，逐位一致。
- LLM 注入点不变（`Advance` 内含 Narrate/FateEvent 调用，同 `Run` 现状）。

---

## 5. 问题 3：UI 层设计（Compose，核心卖点是创伤可视化）

暗色主题 + 终端复古点缀（等宽字体标签、PICO-8 式色板参考姊妹项目）——延续 rebirth 的终端气质但移动端原生。

### 5.1 屏 1：主页/族谱
- 当前代际数 + 遗传敏感性条（0~1）+ 继承天赋徽章。
- 历代墓志铭卡片列表（移动端新增 `lineage.json` 记录每代结局摘要，比 CLI 的单项血统更丰富，P1 可选项）。
- 「开始新的人生」主按钮；种子显示/复制入口（确定性卖点：分享种子 = 分享人生）。

### 5.2 屏 2：角色创建（三段式）
- **出生**：3 张卡片（名称/描述/属性加成/敏感性 +Δ），单选。
- **天赋**：10 格网格，稀有度星标（common/rare/epic/legendary → 星徽章 + 色阶），保底稀有发光提示，选 3，防重复。
- **属性**：20 点分配——4 条滑杆（颜值/智力/体质/家境），实时「剩余 N 点」计数，快捷预设按钮（5/5/5/5、偏科流）。
- 血统继承提示条：当前遗传敏感性 + 继承天赋（如第 3 代）。

### 5.3 屏 3：人生时间线（主玩法屏）
- `LazyColumn` 年份卡片流：`[N 岁]` 徽章 + 职业 chip + 事件文本（2 行截断可展开）。卡片徽标：
  - ⚡ 创伤事件（alpha 值）、🩹 疗愈事件（q 值）、✨ LLM 润色、🎲 LLM 命运事件、🔴 病理态年份（红色左边框 + 背景淡红）。
  - 入行/离职/退休卡特殊样式；死亡卡 = 全屏结语过渡。
- 交互：底部 FAB「下一年」+ 自动播放开关（500ms/1s/2s 三档）+ 年份点击 → 详情 sheet（属性变化 Δ、当时创伤状态、AR(1) 运势值）。
- 新事件自动滚动；步进中「命运编织中」骨架 shimmer 对应 LLM 阻塞窗口。

### 5.4 屏 4：创伤面板（核心卖点，Compose Canvas 手绘，零图表依赖）
- **三线轨迹图**：x=年龄，y=[0,1]。M（记忆痕迹）、A（杏仁核）、P（前额叶）三条线；粗线叠加负荷 L=0.6M+0.4A。
- **迟滞区间可视化**：EnterAt=0.80（红虚线）、ExitAt=0.35（绿虚线）两条阈值线；两线之间填充「迟滞带」；L 线穿过 0.80 进入红区 = 病理态闩锁点亮，需跌回 0.35 以下才熄灭——图上直接画出一段病理期示例注释（这是游戏唯一可视化讲解迟滞概念的地方，值得做精致）。
- **鞍结分岔仪表**：小型 S 曲线（双稳态示意）+ 当前负荷点的位置 + 进出阈值箭头，配一行文字「负荷 ≥ 0.80 进入病理态，< 0.35 才退出」。
- **结局页**：墓志铭卡片（LLM 或兜底）、死因 chip、终局五维属性条、敏感性遗传可视化（父代 → 子代条形图，标注 ψ=0.7 亚加性衰减公式）、「开启下一代」按钮（自动携种子+1 代）。
- 病理态全程主题联动：时间线红晕、状态栏负荷计。

---

## 6. 问题 4：LLM 层移植

### 6.1 可行性结论：`internal/llm` 直接复用
- `http.Client`（纯 Go）在安卓进程内正常工作；`crypto/tls` 读系统根证书（Go 对安卓系统信任库有内置支持）。⚠️需验证：真机 HTTPS + 纯 Go DNS 解析（`CGO_ENABLED=0` 走 netgo 解析器读 /etc/resolv.conf；Termux 的 Go 程序先例可行，但私有 DNS/DoT 环境下需真机冒烟一次）。
- key 注入路径天然干净：`llm.New(key, model)` 由 Kotlin 从 Keystore 取出传入 `new_session` 的 `narrator.key` 字段——Go 包本身零改动，key 只存在于进程内存与协议行中。
- 预算/超时策略**原样复用**：`DefaultCallBudget=24`/局、Narrate 12s、FateEvent 18s、Epitaph 12s、schema 校验 + 纯文本兜底 + `stripControl`。移动端唯一新增：`next` 阻塞期间 UI 显示「命运编织中」（替代 ANSI hint）。

### 6.2 安全存储
- Android Keystore 主密钥 + AES-GCM 加密 key（复用姊妹项目 `api_checkers_master` 模式，代码可直接借鉴）。
- 设置页：provider 预设下拉（openrouter/deepseek/自定义 URL）、model、key 输入、`max_age`、`narrate_ratio`、LLM 开关、调用预算显示。
- 日志红线：Go/Kotlin 双端日志禁止回显 key 与完整请求体（协议层就不含 key 的回显路径）。

### 6.3 LLM 与重放恢复的冲突（CacheNarrator）
LLM 输出不确定，重放恢复时不能重新调用。设计：`session.json` 内嵌 `llm_cache`（`{age: {fate?: Event, texts: {eventID: text}}}`，每局 ≤24 条，KB 级）。恢复时用 `CacheNarrator` 包装真 Narrator：命中缓存直接返回缓存文本/事件（保证 used 集合与属性变化逐位一致），未命中（未来年份）转发真实调用。fate 事件仅在 `age%10==0 && age>=20` 触发 → 以 age 为键无歧义。

---

## 7. 问题 5：确定性保证（跨平台复现）

| 层面 | 结论 | 依据 |
|---|---|---|
| `math/rand` 序列 | **同种子同序列，跨 OS/架构/Go 版本稳定** | 旧版 `math/rand` 算法为纯 Go 整数实现（无汇编路径）；Go 团队为不动旧序列而另立 `rand/v2`，旧序列按 Go 1 兼容承诺冻结。项目用的是旧版 `rand.NewSource` ✓ |
| 浮点 | **跨架构逐位一致** | Go 规范要求 IEEE 754 正确舍入（gc 工具链 amd64 走 SSE2、arm64 走 NEON 均为严格 IEEE，无 x87 扩展精度问题）；代码只用 `+ * math.Max/Min` |
| map 遍历序 | 无风险 | `facts`/`used`/`tierWeights` 只查不遍历；加权抽取全部走 slice |
| 隐患检查 | 通过 | 无 `crypto/rand`、无 `time.Now`（核心路径；仅 LLM fate 事件 ID 用 `time.Now`，属非确定性分支，重放时走缓存绕开） |
| 默认 seed=0 | 时间播种，**本就不复现**（与 CLI 一致） | 移动端设置页提供显式 seed 输入 + 「复制本局种子」，复现玩法明确化 |

**锁定机制（golden 测试）**：
1. 新增 `game` 测试：固定 seed 跑完整一局，对 `Result.History` 哈希断言——桌面 CI 锁定；
2. P0 验收：真机 daemon 与桌面 CLI 同 seed 同选择，History 逐字节 diff 为空（§2.1 协议使 daemon 可在桌面跑，测试全程 CI 可覆盖）；
3. F-Droid 侧固定 Go 工具链版本 → 字节级可复现构建进一步兜底。

---

## 8. 问题 6：F-Droid 发布路径（完整 checklist）

### 8.1 合规预检（对照姊妹项目经验）
- [x] MIT LICENSE（仓库根已有）；[x] 零依赖（Go 侧）/ 纯 FOSS 依赖（androidx/compose 侧）
- [x] 单一 `INTERNET` 权限；`usesCleartextTraffic=false`；`allowBackup=false`（+ dataExtractionRules，姊妹项目 lint 已踩过）
- [x] 无遥测/广告/支付；key 进 Keystore
- [ ] **git tag**（当前零 tag → 发布前置）· [ ] fastlane 元数据 · [ ] fdroiddata MR

### 8.2 反特性声明（NonFreeNet）
LLM 叙事依赖专有服务（OpenRouter/DeepSeek），但**游戏可完全离线游玩**（叙事关闭时零网络请求）。建议仍如实声明（宁可声明后由 reviewer 裁量，也强于被发现后补）：`metadata` 中 `AntiFeatures: NonFreeNet`，full_description 附一句：

> （en）"Optional AI narration uses proprietary LLM services (OpenRouter / DeepSeek) and is disabled by default offline. The game is fully playable without any network access."
> （zh）「可选 AI 叙事依赖专有大模型服务（OpenRouter/DeepSeek），关闭后游戏完全离线可玩。」

另需在描述中注明：**含成人主题文本内容**（成人行业/邪教/暴力等事件，纯文字无图像）——F-Droid 无年龄分级机制，如实披露降低审核摩擦。

### 8.3 仓库侧
1. 打 tag（发布时）`v0.8.0`，versionCode/versionName 在 `android/app/build.gradle.kts`（`UpdateCheckMode: Tags` 可提取）
2. `android/` 子目录为 Android 工程（**单仓单 tag 双端同版**；备选：独立仓库 + git submodule 引 Go 核心——fdroiddata 支持 submodule，但单仓更简单）
3. 提交 gradle wrapper；JDK17 + AGP/Compose 版本沿用姊妹项目已验证组合（Kotlin 2.0.21/AGP 8.5.2/Gradle 8.9，⚠️需验证当时最新可用组合）
4. `android/scripts/fetch-go.sh`：下载**固定版本** Go tarball（含 SHA-256 校验，幂等，缓存到 build 目录）
5. `android/scripts/build-core.sh`：4 ABI 交叉编译 → jniLibs：
   ```bash
   for abi in arm64-v8a armeabi-v7a x86 x86_64; do
     GOOS=android GOARCH=$goarch CGO_ENABLED=0 GOFLAGS="-trimpath" \
       go build -buildvcs=false -ldflags="-s -w" \
       -o android/app/src/main/jniLibs/$abi/librebirth_core.so ./cmd/mobile
   done
   ```
   （`-trimpath -buildvcs=false` 是二进制可复现的关键；无 NDK 参与）
6. `isMinifyEnabled=false`（无 R8，无 keep 规则负担，沿用已验证经验）
7. fastlane 元数据：`fastlane/metadata/android/en-US/` + `zh-CN/`：short_description（<80 字符）、full_description、icon.png（512）、phoneScreenshots×2（真机实截，禁占位图上线）、changelogs/<versionCode>.txt（≤500 字符）
8. 可复现验证脚本 `verify-reproducible.sh`：两个不同路径干净 checkout 各构建一次 → `sha256sum` APK 对比（姊妹项目同法已实测通过；我们的增量是 Go 二进制，Go 构建本身确定性由固定工具链 + trimpath 保证）
9. 签名决策（**时间敏感**）：Verified 徽章需自有 keystore——**首次发布前**生成并离线备份（Android 不允许中途换签名）；apksigner 用 build-tools 34（35+ 有 apksigcopier 兼容问题，姊妹项目已踩坑）

### 8.4 fdroiddata 草稿（结构示意，具体字段以官方 schema 为准）

```yaml
Categories: [Games]
License: MIT
SourceCode: https://github.com/xieguaiwu/rebirth
IssueTracker: https://github.com/xieguaiwu/rebirth/issues
Changelog: https://github.com/xieguaiwu/rebirth/releases
AutoName: 重生 Rebirth
AntiFeatures: [NonFreeNet]
RepoType: git
Repo: https://github.com/xieguaiwu/rebirth

Builds:
  - versionName: 0.8.0
    versionCode: 800
    commit: v0.8.0
    subdir: android
    gradle: [yes]
    prebuild:
      - bash fetch-go.sh        # 下载固定版本 Go + SHA-256 校验（buildserver 未预装 Go，需验证）
    build:
      - bash build-core.sh      # 4 ABI 交叉编译进 jniLibs（gradle 阶段打包）
#   ndk: 无（方案 C 不需要；若切方案 A 则需 ndk: r27 等固定版本）

AutoUpdateMode: Version v%v
UpdateCheckMode: Tags
# Verified 徽章路线（可选，需自有签名）：
# Binaries: https://github.com/xieguaiwu/rebirth/releases/download/v%v/rebirth-v%v.apk
# AllowedAPKSigningKeys: <apksigner verify --print-certs 的证书 SHA-256>
```

⚠️需验证：buildserver 是否已预装 golang（大概率没有 → fetch-go.sh 兜底，prebuild 在容器内以非 root 运行，tarball 下载为标准做法）；`subdir` + 相对脚本路径的精确行为用本地 `fdroid build` 预演。

### 8.5 提交路径（沿用姊妹项目经验）
fork `gitlab.com/fdroid/fdroiddata` → 新增 `metadata/com.xieguiawu.rebirth.yml` → MR → GitLab CI 自动 lint + 构建 → 维护者 review（排队 1~4 周）→ 合并后 24~48h 上架。发版纪律：bump versionCode/Name → tag → changelogs 更新（与姊妹项目 Phase 3 同规）。

---

## 9. 问题 7：分阶段路线图

| 阶段 | 内容 | 验收标准 | 估时 |
|---|---|---|---|
| **P0 可玩 MVP**（离线、无 LLM） | ①会话化重构（§4.4）+ golden 测试 ②cmd/mobile daemon + JSON 协议 ③Android 工程 + 进程桥（§2）④Compose 四屏 + 创伤面板（§5）⑤血统存档 + session.json 恢复 | ①真机完整一局 0~100 岁步进，无崩溃/ANR ②**同种子同选择：真机 History == 桌面 CLI History（逐字节）** ③杀进程重开 → 恢复到正确年份（重放）④`go test ./...` 全绿 + 新 session 测试 ⑤APK（4 ABI）≤ ~15MB；飞行模式全程可玩、零网络请求 | 4~6 人日 |
| **P1 LLM 接入** | ①设置页 + Keystore 密钥 ②CacheNarrator + 重放缓存 ③真机 LLM 冒烟（deepseek-v4-flash 已验证可用）④族谱 lineage.json + 血统导出（分享 sheet） | ①真机 DeepSeek 叙事/命运/墓志铭各 ≥1 次成功 ②断网全回退本地文本，零崩溃 ③预算 24 次/局在真机生效；日志无 key ④后台 10 分钟进程被杀 → 恢复正确 ⑤设置项与桌面 config.json 字段对齐 | 2~3 人日 |
| **P2 商店发布** | ①fastlane 元数据 + 真机截图 ②verify-reproducible.sh 双构建 SHA-256 一致 ③keystore 生成+离线备份+GitHub Releases 签名 APK ④tag vX.Y.Z ⑤fdroiddata MR | ①本地 `fdroid build`/lint 通过 ②双构建 SHA-256 一致 ③MR 提交且 CI 绿 ④review 无 blocker；合并后上架 ⑤NonFreeNet + 成人内容披露就位 | 1~2 人日 + 排队 1~4 周（外部） |

---

## 10. 问题 8：风险与权衡

| # | 风险 | 等级 | 缓解 |
|---|---|---|---|
| 1 | exec 从 nativeLibraryDir 启动在 targetSdk 35 真机不可行（⚠️需验证） | 高 | P0 第一天真机冒烟；失败 → 切方案 A（共享 Session API，~1 人日）；最后手段 targetSdk 降级（不推荐，F-Droid 可能接受但属坏味道） |
| 2 | F-Droid buildserver 无 Go 工具链（⚠️需验证） | 中 | fetch-go.sh 固定版本 + SHA-256；`fdroid build` 本地预演 |
| 3 | 真机 HTTPS/DNS 在私有 DNS 环境失败（⚠️需验证） | 中 | P1 冒烟；兜底：允许自定义 BaseURL 走用户自建代理端点 |
| 4 | 进程生命周期：后台被杀丢进度 | 中 | session.json 每 year 落盘 + 重放恢复（确定性核心使重放精确）；前台游玩为主场景 |
| 5 | gomobile 路径坑（若启用 A）：gobind 类型限制、与 Go 版本锁步、R8 keep、cgo 可复现性 | 中 | 仅作备选；坑清单见 §3 |
| 6 | 成人内容审核摩擦 | 中 | 如实披露（§8.2）；可选设置「内容过滤」开关（P1+） |
| 7 | NonFreeNet 徽章观感 | 低 | 如实声明（姊妹项目同策）；描述强调「可完全离线」 |
| 8 | keystore 丢失 = 永久无法更新 | 高 | 首次发布前生成 + 双介质离线备份；决策窗口一次性 |
| 9 | fdroiddata review 排队时间不可控 | 中 | 提前 MR；P2 与 P0/P1 并行准备元数据草稿 |
| 10 | apksigner build-tools ≥35 与 apksigcopier 兼容问题 | 低 | 固定 build-tools 34（姊妹项目已验证） |
| 11 | 双端行为漂移 | 低（方案 C 下≈0） | 单引擎 + golden 测试 + 单一 tag 双端同版 |
| 12 | 二进制体积（F-Droid 通用包 4 ABI ~10MB） | 低 | `-ldflags="-s -w"`；评估裁掉 x86/x86_64（模拟器用户受损，默认保留） |
| 13 | 内容热更新 vs F-Droid review 延迟 | 低 | 可选 filesDir 事件分片追加机制（§4.2），非 MVP |

---

## 11. 附录：目标仓库结构

```
rebirth/
├── cmd/
│   ├── rebirth/            # 现 main.go 平移（CLI 行为不变）
│   └── mobile/             # 新增：JSON-lines daemon（平台无关，桌面可测）
├── internal/
│   ├── game/session.go     # 新增：会话化步进器 + 重放（§4.4）
│   ├── game/…              # 现有文件仅 Run 重写为薄循环，其余不动
│   ├── llm/…               # 不动（+ 可选 CacheNarrator，放 mobile 侧或 llm 内）
│   ├── config/…            # 不动（mobile 绕过 DefaultPath）
│   └── tui/                # 保持不动（仅 CLI 用，安卓不编译）
├── android/                # 新增：单 Activity Compose 工程
│   ├── app/build.gradle.kts            # versionCode/Name 唯一真源
│   ├── app/src/main/jniLibs/<abi>/librebirth_core.so   # 构建产物（gitignore）
│   ├── app/src/main/java/com/xieguiawu/rebirth/…       # UI 四屏 + CoreProcess 桥
│   ├── scripts/{fetch-go.sh,build-core.sh,verify-reproducible.sh}
│   └── fastlane/metadata/android/{en-US,zh-CN}/…
├── docs/plans/2026-08-24-rebirth-android-port-plan.md   # 本文档
└── internal/game/data/*.json           # 内容单一真源（embed，双端共享）
```

---

*版本：v1.0 · 作者：ultrabrain · 状态：待评审执行。所有 ⚠️需验证 项在 P0/P1 验收中逐一关闭。*
