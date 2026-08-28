package main

import (
	"DontCrack/config"
	"DontCrack/core"
	"flag"
)

// FasterEdge - 对称、可靠、安全的多场景边缘计算框架
// https://github.com/FasterEdge
// DontCrack Windows 版
// https://github.com/FasterEdge/DontCrack4Windows
// tyza66
// https://github.com/tyza66
// 程序入口
func main() {
	// 系统版本号
	version := "1.0.20260826" // 当前系统的版本

	// 全局配置信息获得
	// 管理子进程相关
	path := flag.String("path", "", "要管理的程序路径（必选，支持可执行文件、.bat/.cmd/.ps1 脚本）")                                  // 程序路径
	args := flag.String("args", "", "传递给程序的参数（可选，默认为空）")                                                       // 程序启动参数
	pre := flag.String("pre", "", "启动前要执行的命令（可选，在 cmd.exe 中执行，可用 && 连接多条命令，默认为空）")                            // 启动前命令
	env := flag.String("env", "", "为子进程追加环境变量，如: \"PATH=C:\\Windows\\System32;FOO=bar\"；用空格或分号分隔(可选，默认为空)")    // 子进程环境变量
	autoRestart := flag.Bool("auto-restart", false, "是否自动重启（可选，默认false）")                                            // 是否开启自动重启
	maxRetries := flag.Int("max-retries", 3, "最大重试次数（可选，-1表示无限次，默认3次）")                                              // 重启最大重试次数
	startNow := flag.Bool("start-now", false, "是否立即启动（可选，默认false）")                                                  // 是否立即启动子进程
	// 健康探针（可选，留空则不启用）
	probeCmd := flag.String("probe-cmd", "", "子进程健康检查命令（如 \"powershell -Command Test-NetConnection 127.0.0.1 -Port 80\"），留空禁用") // 探针命令
	probeInterval := flag.Int("probe-interval", 30, "探针间隔秒数（可选，默认30）")                                                       // 探针间隔
	probeTimeout := flag.Int("probe-timeout", 5, "探针超时秒数（可选，默认5）")                                                            // 探针超时
	probeFailureLimit := flag.Int("probe-failure-limit", 3, "连续失败多少次判定为不健康（可选，默认3）")                                   // 探针失败阈值
	// 管理器相关
	port := flag.Int("port", 11883, "HTTP服务端口(可选，默认11883)")                                                       // 管理端口、mcp连接端口
	listenAddress := flag.String("listen-address", "127.0.0.1", "HTTP监听地址（可选，默认127.0.0.1仅本机访问；对外监听必须配置密码）")
	password := flag.String("password", "", "管理进程的密码（可选，默认为空且不开启密码保护）")                                                // 管理密码
	logCapacity := flag.Int("log-capacity", 200, "日志缓存的最大行数（可选，默认200)")                                                   // 日志缓存的最大行数
	logMaxLineBytes := flag.Int("log-max-line-bytes", 1048576, "单行日志的最大字节数（可选，用于bufio.Scanner，默认1MiB）")                       // 单行日志的最大字节数
	fileLogEnabled := flag.Bool("file-log", false, "是否启用文件日志（可选，默认为false）")                                                // 是否启用文件日志
	localLogPath := flag.String("log-path", "logs\\proc_manager\\", "本地日志文件目录（可选，默认 logs\\proc_manager\\进程名）") // 本地日志文件目录
	localLogLifeDay := flag.Int("log-life-day", 7, "本地日志文件保存天数（可选，默认7天）")                                                  // 本地日志文件保存天数

	// 解析传入的参数
	flag.Parse()

	// 将传入的配置信息转换为全局配置结构体
	// 安全缺省: 非环回监听时必须配置密码，否则管理接口会暴露给整个网络
	if *listenAddress != "127.0.0.1" && *listenAddress != "localhost" && *listenAddress != "::1" && *password == "" {
		flag.Usage()
		panic("对外监听(" + *listenAddress + ")必须通过 -password 配置管理密码，否则请保持默认 127.0.0.1")
	}

	config := config.ParseConfigListenWithProbe(version, *path, *args, *pre, *env, *autoRestart,
		*maxRetries, *startNow, *port, *listenAddress, *password, *logCapacity, *logMaxLineBytes,
		*fileLogEnabled, *localLogPath, *localLogLifeDay,
		*probeCmd, *probeInterval, *probeTimeout, *probeFailureLimit)

	// 携带启动参数启动管理器
	core.Start(*config)
}