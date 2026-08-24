# rebirth 移动端协议 v1（冻结契约）

> 状态：**冻结**。Go 核心（cmd/mobile）与 Android 侧（进程桥）都以此文档为唯一契约。
> 任何偏离必须先改本文档再改代码；并行开发期间如有冲突，由集成阶段裁决。

## 0. 总则

- 传输：stdin/stdout **逐行 JSON**（每行一个对象，`\n` 分隔）。
- 请求：`{"id": <int>, "cmd": "<name>", ...参数}`；`id` 由客户端自增。
- 响应：`{"id": <int>, "ok": true, "data": {...}}` 或 `{"id": <int>, "ok": false, "error": "<消息>"}`。
- 顺序：客户端串行发送，服务端按序应答。不允许并发请求。
- stderr：仅日志（Go 侧 `log` 包），**禁止输出 key / 完整请求体**。
- 启动参数：`cmd/mobile --dir=<dataDir>`（如 `/data/user/0/com.xieguaiwu.rebirth/files/rebirth`）。
  所有存档路径由 `--dir` 派生：`bloodline.json`、`session.json`。
- 版本：`ver` 字段 = Go 侧版本号（当前 "0.10.0"），`proto` = 1。

## 1. 命令

### 1.1 `hello`

请求：`{"id":1,"cmd":"hello"}`
响应 data：`{"ver":"0.10.0","proto":1}`

### 1.2 `bloodline_get`

请求：`{"id":2,"cmd":"bloodline_get"}`
响应 data：`{"generation":0,"sensitivity":0.0,"inherited_talent":""}`
（无存档 → generation 0，不报错）

### 1.3 `draw_births`

请求：`{"id":3,"cmd":"draw_births","seed":12345,"lang":"zh"}`（lang 可选，默认 zh）
响应 data：`{"births":[<Birth>,<Birth>,<Birth>]}`（3 张，加权无重复）

`Birth` = `{"id":"slum","name":"贫民窟","desc":"...","weight":10,"bonus":{"chr":0,"int":0,"str":0,"mny":0,"spr":0},"sensitivity_add":0.05}`
（bonus 缺省字段 = 0；字段名与 game.Birth JSON tag 一致）

### 1.4 `draw_talents`

请求：`{"id":4,"cmd":"draw_talents","seed":12345,"lang":"zh"}`（lang 可选，默认 zh）
响应 data：`{"talents":[<Talent>×10]}`（10 张，含稀有保底）

`Talent` = `{"name":"乐天派","desc":"...","rarity":"common","bonus":{...},"trauma_mult":1,"luck_bonus":0,"therapy_mult":1,"inheritable":false}`

### 1.5 `new_session`

请求：

```json
{
  "id": 5, "cmd": "new_session",
  "seed": 12345,
  "lang": "zh",
  "birth": {<Birth> 或 null},
  "talents": [<Talent>×3],
  "points": {"chr":5,"int":5,"str":5,"mny":5},
  "max_age": 100,
  "narrator": {
    "enabled": true,
    "providers": [
      {"provider":"deepseek","model":"deepseek-v4-flash","url":"","key":"sk-..."},
      {"provider":"openrouter","model":"","url":"","key":"..."}
    ],
    "budget": 24,
    "ratio": 0.5
  },
  "trauma_overrides": null
}
```

语义：
- `lang`：`"zh"` | `"en"`。决定数据集（data/ 或 data_en/）与 LLM 提示语言。
- `narrator.enabled=false` 或 `providers` 为空数组 → 纯本地（game.Noop）。
- `providers` **有序**：按数组顺序逐个尝试（failover）。每个 provider 独立熔断
  （连续 3 败 → 本世余下跳过该 provider）；全部失败 → 回退本地文本。
  `url` 为空 = 用 provider 预设端点。`provider` 未知但 url 非空 → 视为 OpenAI 兼容自定义端点。
- `budget`：本局 LLM 调用总预算（默认 24），跨 providers 共享；墓志铭免预算。
- `ratio`：narrate 采样比例（同桌面 narrate_ratio，0.5 = 一半事件润色；0 = 全部，1 = 全部）。
- `trauma_overrides`：`{"enter_at":0.8,"exit_at":0.35,"drive":0.65,"event_trauma_scale":0.5}`（均可缺省 = 不动）。

响应 data：`{"generation":1}`（第几代）

**血统继承**：daemon 从 `bloodline.json` 读取 generation/sensitivity/inherited_talent，
curGen = generation+1；`inherited_talent` 按名查找（跨语言找不到 → 无继承天赋，不报错）。

### 1.6 `next`

请求：`{"id":6,"cmd":"next"}`
响应 data（每年一条，直到 died:true）：

```json
{
  "age": 7,
  "lines": ["[  7 岁] 高年级的孩子堵在巷口，你学会了绕远路回家。"],
  "career": {"id":"factory_worker","name":"工厂工人"},
  "career_change": "enter",
  "event": {
    "id":"bully_01","text":"高年级的孩子堵在巷口，你学会了绕远路回家。",
    "good":false,"llm":false,
    "trauma_alpha":0.3,"therapy_q":0,
    "delta":{"chr":0,"int":0,"str":0,"mny":0,"spr":-1}
  },
  "stats": {"chr":5.2,"int":6.0,"str":4.8,"mny":3.1,"spr":7.4},
  "trauma": {"m":0.31,"a":0.42,"p":0.63,"load":0.35,"pathological":false},
  "luck": 0.12,
  "llm_broken_notice": false,
  "died": false
}
```

字段语义：
- `lines`：该年全部已记录行（UI 直接渲染；与桌面 CLI 逐字节同源）。
- `career`：当前职业对象；**无职业（含无业/退休后）= null（字段缺省）**。
- `career_change`：`"enter"`（入行）| `"quit"`（离开）| `"retire"`（退休）| null。
- `event`：本年度事件（无事件年份 = null）。`llm` = 是否 LLM 润色/注入。
- `stats`：五维属性 [0,10]（chr 颜值 / int 智力 / str 体质 / mny 家境 / spr 快乐）。
- `trauma`：`m` 记忆痕迹 / `a` 杏仁核 / `p` 前额叶 / `load` 负荷 / `pathological` 病理态。
- `luck`：本年度 AR(1) 运势 [-1,1]。
- `llm_broken_notice`：本年度是否打印过熔断提示（一次性）。

死亡年响应 data 增加：

```json
{
  "died": true,
  "death_status": "长期抑郁",
  "epitaph": "一生至此。",
  "lineage_saved": true,
  "next_generation": 2,
  "next_sensitivity": 0.76
}
```

- `death_status`：幼年夭折 / 身体耗竭 / 未成年早逝 / 长期抑郁 / 安详离世（+「（终生生处于创伤病理态）」后缀按原逻辑）。
- daemon 在死亡时自动保存 `bloodline.json`（与桌面同 schema）。`next` 在 died:true 后再调用 → 报错 `"session finished"`。

### 1.7 `checkpoint_get`

请求：`{"id":7,"cmd":"checkpoint_get"}`
响应 data：`{"exists":true,"age":45,"generation":2}`（无存档 → exists:false，不报错）

### 1.8 `resume_session`

请求：

```json
{"id":8,"cmd":"resume_session","narrator":{"enabled":true,"providers":[...],"budget":24,"ratio":0.5}}
```

- 读取 `session.json`（含 cfg 全量 + llm_cache + 已推进年龄），重放至已保存年龄（确定性，本地瞬间完成），后续 `next` 从该年龄继续。
- narrator 配置**必须由客户端重传**（key 永不落盘）；session.json 内不含任何 key。
- 无 checkpoint → 报错 `"no checkpoint"`。

### 1.9 `shutdown`

请求：`{"id":9,"cmd":"shutdown"}` → 响应 `{"ok":true}` 后进程退出。

## 2. checkpoint 与恢复语义（session.json）

- 每次 `next` 成功后原子写入 `session.json`（JSON：`{"cfg":{...},"age":N,"llm_cache":{...}}`）。
- `cfg` 含：seed / lang / birth / talents / points / max_age / trauma_overrides / bloodline（本局继承值）/ narrator 的**非敏感部分**（providers 的 provider/model/url，**无 key**）。
- `llm_cache`：`{"<age>": {"fate": {<Event>}, "texts": {"<eventID>": "<text>"}}}`。
- 重放 = `NewSession(cfg)` + 循环 `Advance` 到 age（CacheNarrator 命中缓存，未命中转真调用）。
- 恢复后 narrator 以客户端传入的 providers 重建；熔断/预算状态从零开始（可接受）。

## 3. 错误约定

- 未知 cmd → `{"ok":false,"error":"unknown command <name>"}`。
- 参数校验失败 → `{"ok":false,"error":"<具体原因>"}`。
- 内部 panic → 进程崩溃由 Android 侧重启（ProcessBuilder 重启 + checkpoint 恢复）。

## 4. 日志红线

- Go 侧 stderr 日志与 Android 侧 Logcat **禁止**输出：API key、Authorization 头、完整请求体。
- 错误消息若包含 HTTP 响应体，仅截断前 200 字符且不含 Authorization。
