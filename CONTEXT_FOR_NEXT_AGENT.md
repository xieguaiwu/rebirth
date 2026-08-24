# CONTEXT_FOR_NEXT_AGENT.md

最后更新: 2026-08-25 00:10

## 项目当前状态

rebirth v0.10.0 —— Go 终端人生重开模拟器 + **安卓客户端（android/ 子目录）**，可玩、可构建、全部测试绿、确定性跨端锁定、F-Droid 发布材料就绪。

- CLI 二进制: `~/.local/bin/rebirth`（每次改动后重新构建部署）
- 仓库: https://github.com/xieguaiwu/rebirth（public，master，**未打 tag v0.10.0**）
- 数据: 26 职业 / 13 出身 / 63 天赋 / 339 事件（9 分片）× **双语（data/ 中文 + data_en/ 英文）**
- 安卓: `android/` 子目录（单仓单 tag 双端同版），arm64-only APK ~9MB

## 架构（v0.10.0 新增）

```
main.go                 入口：flags、出生/天赋/属性点流程、步进模式接线、--lang
internal/game/
  session.go            ★ Session 可恢复步进器：NewSession/Advance/DeathCheck/
                        Finish/Result；Run 重写为薄循环（输出逐字节一致，
                        TestRunSessionByteIdentical + Golden 哈希锁定）
  run.go                薄驱动 + Narrator 接口/Noop/Config/Result
  events.go             双语 embed（data/ + data_en/），LoadEventsLang(lang)
  career.go             LoadCareersLang/LoadBirthsLang（同 schema）
  trauma.go             创伤动力学（未动）
internal/llm/
  llm.go                Narrator + Lang 字段（提示词 zh/en）+ ChainNarrator
                        （有序 failover、每 provider 独立熔断、共享预算、
                        墓志铭免预算）
cmd/mobile/main.go      ★ JSON-lines daemon（契约 docs/mobile-protocol.md）：
                        hello/bloodline_get/draw_births/draw_talents/new_session/
                        next/checkpoint_get/resume_session/shutdown；--dir 参数；
                        每 year 原子写 session.json（cfg+llm_cache，无 key）；
                        死亡自动存 bloodline.json；重放恢复确定性（e2e 测试锁定）
android/                ★ 单 Activity Compose 工程（com.xieguaiwu.rebirth）
  app/src/main/java/…/core/   CoreProcess（exec .so + JSON-lines + 30s 超时 +
                              崩溃重启 + stderr 脱敏）、Protocol.kt（协议模型）、
                              FakeCore（测试注入）
  …/ui/                 Home/Create/Timeline/TraumaPanel/Settings 五屏
  …/security/           Keystore AES-GCM（key 永不离机）+ InMemory 测试实现
  app/src/main/jniLibs/arm64-v8a/librebirth_core.so（构建产物，gitignore）
  scripts/              fetch-go.sh（固定 Go 工具链）/ build-core.sh（arm64 纯
                        Go 交叉编译）/ verify-reproducible.sh（双构建 SHA 比对）
  fastlane/             双语元数据（en-US/zh-CN，截图是占位待替换）
docs/mobile-protocol.md ★ 冻结协议契约 v1
docs/fdroiddata/com.xieguaiwu.rebirth.yml   fdroiddata 草稿
```

## 关键决策与已验证事实

1. **ABI 只出 arm64-v8a**：Go 1.25 实测（官方 + Fedora 工具链）android/amd64、arm、386 全部要求 cgo 外部链接；arm64 是唯一纯 Go 目标。cgo + NDK 会破坏 F-Droid 可复现性 → 放弃 32 位/x86_64。
2. **可复现性已实测**：verify-reproducible.sh 双构建 SHA-256 一致（20c07627...）。
3. **checkpoint 重放 off-by-one 教训**：checkpoint 在 Advance(age N) 之后保存 → resume 重放必须 `for Age <= cp.Age`（含 N），否则 N 岁会被处理两次（e2e 测试抓出）。
4. **llm 测试服务器格式坑**：client 解析 OpenAI 格式 `{choices:[{message:{content}}]}`，测试 fake server 直接返回 `{"text":...}` 会让所有调用静默失败 → e2e fakeServer 已按 OpenAI 格式。
5. **根 .gitignore 裸模式 `rebirth` 曾静默忽略 Kotlin 包目录**（com/xieguaiwu/rebirth/）——已锚定为 `/rebirth`。教训：gitignore 模式要锚定。
6. **android/386/arm 需要 cgo**（Go 1.25）→ 见决策 1。
7. 双语数据事实键（requires/conflict/sets/context）已用脚本验证 zh==en（339 事件零漂移）。

## 版本史要点（v0.10.0）

- v0.10.0（本次）：会话化重构（Session 可恢复步进器，CLI 输出逐字节不变）、cmd/mobile daemon + 冻结协议、ChainNarrator 多供应商 failover（玩家可配多个 LLM 供应商或纯离线）、双语数据 data_en + --lang、安卓客户端（五屏 Compose、进程桥、Keystore key 存储、F-Droid 全套准备）、可复现构建实测通过。

## 待办

- [ ] **真机验证（P0）**：Pixel/arm64 真机侧载 app-release.apk，验证 nativeLibraryDir exec .so 在 targetSdk 35 可行（方案 C 最大风险点，若失败切方案 A gomobile）
- [ ] **真机冒烟**：完整一局 0~100 岁步进无崩溃/ANR；杀进程重开恢复正确年份；飞行模式全程可玩
- [ ] **真机 LLM 冒烟**：DeepSeek key 输入后叙事/命运/墓志铭各 ≥1 次成功；断网全回退本地
- [ ] fastlane 截图替换为真机实截（当前为占位图）
- [ ] 打 tag v0.10.0 + push（需用户确认）；fdroiddata fork + MR
- [ ] 本地 `~/.local/bin/rebirth` 重新部署（v0.10.0 带 --lang）
- [ ] graphify-out 已 gitignore，本地图谱需 `graphify update .` 手动重建
- [ ] 交互模式人工测试（CLI 一直待办）

## 知识图谱

- graphify-out/: 不存在（gitignore），需 `graphify update . --no-llm` 重建

## 最后更新时间

2026-08-25 00:10
