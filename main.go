// rebirth —— 终端人生重开模拟器。
//
// 确定性核心（创伤动力学 + AR(1) 运势 + 加权事件抽取）负责随机性；
// LLM（openrouter/ox-alpha）只做叙事润色与命运事件注入，全部输出经
// schema 校验，失败即回退。跨代血统存档实现亚加性应激敏感性遗传。
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rebirth/internal/game"
	"rebirth/internal/llm"
	"rebirth/internal/tui"
)

var version = "0.6.0"

const pointsTotal = 20

func savePath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "rebirth", "bloodline.json")
	}
	return "bloodline.json"
}

func main() {
	seed := flag.Int64("seed", 0, "随机种子（0=时间播种）")
	auto := flag.Bool("auto", false, "自动模式：所有选择取第一项")
	noLLM := flag.Bool("no-llm", false, "禁用大语言模型叙事")
	step := flag.Bool("step", false, "逐条推进：每条信息等回车（交互终端默认开启；--auto 时忽略）")
	model := flag.String("model", "stealth/ox-alpha", "OpenRouter 模型名")
	maxAge := flag.Int("max-age", 100, "寿命上限")
	showVersion := flag.Bool("version", false, "打印版本号")
	flag.Parse()

	if *showVersion {
		fmt.Println("rebirth", version)
		return
	}
	if *seed == 0 {
		*seed = time.Now().UnixNano() % 1_000_000_007
	}

	evs, err := game.LoadEvents()
	if err != nil {
		fmt.Println("[FAIL] 事件加载:", err)
		os.Exit(1)
	}
	careers, err := game.LoadCareers()
	if err != nil {
		fmt.Println("[FAIL] 职业加载:", err)
		os.Exit(1)
	}
	births, err := game.LoadBirths()
	if err != nil {
		fmt.Println("[FAIL] 出生背景加载:", err)
		os.Exit(1)
	}
	talents, err := game.LoadTalents()
	if err != nil {
		fmt.Println("[FAIL] 天赋加载:", err)
		os.Exit(1)
	}
	bloodline, err := game.LoadBloodline(savePath())
	if err != nil {
		fmt.Println("[WARN] 血统存档损坏，从第 1 代开始。")
	}

	rng := rand.New(rand.NewSource(*seed))
	narrator := buildNarrator(*noLLM, *model)

	birth := pickBirth(births, rng, *auto)
	talentsPick := pickTalents(talents, rng, *auto)
	stats := allocatePoints(rng, *auto)

	// Current generation = stored ancestors + 1. LoadBloodline no longer
	// increments (momus P1-1: the old double-increment skipped every even
	// generation).
	curGen := bloodline.Generation + 1
	cfg := game.Config{
		Seed:       *seed,
		Birth:      birth,
		Bloodline:  &game.Bloodline{Generation: curGen, Sensitivity: bloodline.Sensitivity, InheritedTal: bloodline.InheritedTal},
		Talents:    talentsPick,
		InheritTal: inheritTalent(talents, bloodline),
		LLM:        narrator,
		MaxAge:     *maxAge,
	}.WithPoints(stats[0], stats[1], stats[2], stats[3])

	// Step mode: opt-in via --step, on by default on interactive terminals,
	// always off in auto mode.
	cfg.Step = !*auto && (tui.IsTTY() || *step)
	if cfg.Step {
		cfg.Pause = func() bool {
			fmt.Print("\n\033[2m回车=下一年 · q=退出\033[0m ")
			line, err := tui.ReadLineErr("")
			if err != nil {
				return true // Ctrl+C / Ctrl+D / EOF: leave the life
			}
			switch strings.TrimSpace(line) {
			case "q", "quit", "exit", "退出":
				return true
			}
			return false
		}
	}

	res, err := game.Run(os.Stdout, cfg, evs, careers)
	if err != nil {
		fmt.Println("[FAIL] 模拟中断:", err)
		os.Exit(1)
	}

	next := &game.Bloodline{
		Generation:  curGen,
		Sensitivity: game.InheritSensitivity(res.Sensitivity, (rng.Float64()*2-1)*0.1, 0.7),
	}
	if len(talentsPick) > 0 && talentsPick[0].Inheritable {
		next.InheritedTal = talentsPick[0].Name
	}
	if err := next.Save(savePath()); err != nil {
		fmt.Println("[WARN] 存档失败:", err)
	} else {
		fmt.Printf("\n[OK] 第 %d 代已记录（遗传敏感性 %.2f）。\n", next.Generation, next.Sensitivity)
	}
}

func buildNarrator(disabled bool, model string) game.Narrator {
	key := os.Getenv("OPENROUTER_API_KEY")
	if disabled || key == "" {
		if !disabled && key == "" {
			fmt.Println("[WARN] 未设置 OPENROUTER_API_KEY，使用纯本地模式。")
		}
		return game.Noop
	}
	c := llm.New(key, model)
	fmt.Printf("[OK] 叙事层已启用：%s\n", model)
	return llm.NewNarrator(c)
}

func pickBirth(bs []game.Birth, rng *rand.Rand, auto bool) *game.Birth {
	draw := game.DrawBirths(bs, 3, rng)
	if len(draw) == 0 {
		return nil
	}
	fmt.Println("\n── 选择出生 ──")
	for i, b := range draw {
		fmt.Printf("  [%d] %s：%s\n", i+1, b.Name, b.Desc)
	}
	idx := 1
	if auto {
		fmt.Println("  [auto] 选定第 1 项")
	} else {
		var err error
		idx, err = tui.Choose("你的出身是", len(draw))
		if err != nil {
			fmt.Println("\n已退出。")
			os.Exit(0)
		}
	}
	return &draw[idx-1]
}

func pickTalents(ts []game.Talent, rng *rand.Rand, auto bool) []game.Talent {
	draw := game.DrawTalents(ts, 10, rng)
	fmt.Println("\n── 抽取天赋（10 选 3，* 为稀有，** 史诗，*** 传说）──")
	for i, t := range draw {
		fmt.Printf("  [%2d] %s %-8s %s\n", i+1, game.RarityStars(t.Rarity), t.Name, t.Desc)
	}
	picked := make([]game.Talent, 0, 3)
	if auto {
		n := min(3, len(draw)) // momus P3-10: never slice past len(draw)
		picked = append(picked, draw[:n]...)
		for _, t := range picked {
			fmt.Printf("  [auto] 选定：%s\n", t.Name)
		}
		return picked
	}
	for len(picked) < 3 && len(picked) < len(draw) {
		idx, err := tui.Choose(fmt.Sprintf("选第 %d 个天赋", len(picked)+1), len(draw))
		if err != nil {
			fmt.Println("\n已退出。")
			os.Exit(0)
		}
		t := draw[idx-1]
		dup := false
		for _, p := range picked {
			if p.Name == t.Name {
				dup = true
			}
		}
		if dup {
			fmt.Println("该天赋已选过，换一个。")
			continue
		}
		picked = append(picked, t)
	}
	return picked
}

// inheritTalent resolves the bloodline talent by name.
func inheritTalent(all []game.Talent, b *game.Bloodline) *game.Talent {
	if b == nil || b.InheritedTal == "" {
		return nil
	}
	for i := range all {
		if all[i].Name == b.InheritedTal {
			return &all[i]
		}
	}
	return nil
}

// allocatePoints distributes 20 attribute points; interactive input is a
// plain menu (5/5/5/5 default or custom split).
func allocatePoints(rng *rand.Rand, auto bool) []float64 {
	if auto {
		return []float64{5, 5, 5, 5}
	}
	fmt.Println("\n── 分配属性点（共 20 点：颜值/智力/体质/家境）──")
	fmt.Println("回车使用默认 5/5/5/5，或输入四个数如「7 6 5 2」。")
	line := tui.ReadLine("> ")
	var a, b, c, d float64
	if n, _ := fmt.Sscanf(line, "%f %f %f %f", &a, &b, &c, &d); n == 4 {
		sum := a + b + c + d
		// Zero is legal (dump a stat); negatives are not. The runtime
		// init-floor keeps zeroed stats playable instead of instantly dead.
		positive := a >= 0 && b >= 0 && c >= 0 && d >= 0
		if positive && sum <= pointsTotal {
			return []float64{a, b, c, d}
		}
		fmt.Println("[WARN] 数值不可为负且总和不超过 20，已回退默认分配。")
	}
	return []float64{5, 5, 5, 5}
}
