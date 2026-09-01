// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

// runMode 定义子进程的运行模式
// normal: 正常输出直到 lifetime 到期或收到信号退出
// crash: 模拟崩溃退出（退出码非0）
// hang: 挂起不退出，用于测试管理器超时和强杀
// graceful: 等待信号后优雅退出
func main() {
	mode := flag.String("mode", "normal", "运行模式: normal|crash|hang|graceful")
	interval := flag.Duration("interval", time.Second, "输出间隔")
	lifetime := flag.Duration("lifetime", 0, "运行时长，0 表示不主动退出")
	stateFile := flag.String("state-file", "", "用于记录重启计数的文件路径，可选")
	message := flag.String("message", "", "自定义输出前缀")
	flag.Parse()

	logLine := func(format string, args ...any) {
		ts := time.Now().Format("2006/01/02 15:04:05.000000")
		fmt.Printf(ts+" "+format+"\n", args...)
	}

	restartEnv := os.Getenv("RESTART_ENV_COUNT")
	restartCount := parseIntDefault(restartEnv, 0)
	if restartEnv != "" {
		restartCount++
		logLine("env restart count -> %d", restartCount)
	}

	if *stateFile != "" {
		cnt := readCounter(*stateFile)
		cnt++
		_ = os.WriteFile(*stateFile, []byte(fmt.Sprintf("%d", cnt)), 0644)
		logLine("state-file %s count -> %d", *stateFile, cnt)
	}

	logLine("childproc start | pid=%d | mode=%s | interval=%s | lifetime=%s | msg=%s", os.Getpid(), *mode, interval.String(), lifetime.String(), *message)
	logLine("args: %s", strings.Join(os.Args, " "))
	logLine("env EXTRA_INFO=%s", os.Getenv("EXTRA_INFO"))
	logLine("env RESTART_ENV_COUNT=%s", restartEnv)

	// Windows: 用 os.Interrupt (CTRL_BREAK_EVENT / CTRL_C_EVENT) 替代 SIGTERM/SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	start := time.Now()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	crashCode := 42

	for {
		select {
		case <-ctx.Done():
			logLine("received signal, exiting gracefully")
			return
		case t := <-ticker.C:
			logLine("tick at %s", t.Format(time.RFC3339Nano))
			fmt.Fprintf(os.Stderr, "stderr burst at %s\n", t.Format(time.RFC3339Nano))
		default:
			if *lifetime > 0 && time.Since(start) > *lifetime {
				switch *mode {
				case "crash":
					logLine("lifetime reached, simulating crash")
					os.Exit(crashCode)
				case "graceful", "normal":
					logLine("lifetime reached, exiting normally")
					return
				}
			}
			if *mode == "hang" {
				time.Sleep(200 * time.Millisecond)
				if rand.Intn(20) == 0 {
					logLine("hang mode heartbeat pid=%d", os.Getpid())
				}
				continue
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func readCounter(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return v
}