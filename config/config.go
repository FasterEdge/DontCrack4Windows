package config

// FasterEdge - 对称、可靠、安全的多场景边缘计算框架
// https://github.com/FasterEdge
// DontCrack Windows 版
// https://github.com/FasterEdge/DontCrack4Windows
// tyza66
// https://github.com/tyza66
type Config struct {
	// 系统相关
	Version string // 系统版本号(主方法中写死定义)
	// 子进程相关
	Path         string // 打开子进程的可执行文件路径(必填)
	Args         string // 打开子进程所携带的参数(可选)
	Pre          string // 启动前要执行的命令(可选，只能一条但可以是cmd脚本或&&连接的命令)
	Env          string // 子进程的环境变量(可选，如: "PATH=C:\Windows\System32 FOO=bar")
	AutoRestart  bool   // 子进程退出后是否自动重启(可选，默认关闭)
	RestartTimes int    // 最大重试次数(可选，默认3次，-1表示无限次)
	StartNow     bool   // 是否立即启动子进程(可选，默认关闭)
	// 健康探针
	ProbeCmd          string // 探针命令(如 "powershell -Command ...")，空则禁用
	ProbeInterval     int    // 探针间隔(秒，默认30)
	ProbeTimeout      int    // 探针超时(秒，默认5)
	ProbeFailureLimit int    // 连续失败多少次算不健康(默认3)
	// 管理器相关
	Port            int    // 管理进程使用的进程端口(可选，默认11883)
	Password        string // 管理进程使用的密码(可选)
	LogCapacity     int    // 日志容量(可选，默认10MB)
	LogMaxLineBytes int    // 日志单行最大字节数(可选，默认1KB)
	FileLogEnabled  bool   // 是否启用文件日志(可选，默认关闭)
	LocalLogPath    string // 日志文件目录(可选，默认 logs\proc_manager\进程名)
	LocalLogLifeDay int    // 本地日志文件保存天数(可选，默认7天)
}

// 将传入的参数转换为全局配置结构体
func ParseConfig(version string, path string, args string, pre string, env string, autoRestart bool, restartTimes int, startNow bool,
	port int, password string, logCapacity int, logMaxLineBytes int, fileLogEnabled bool, localLogPath string, localLogLifeDay int) *Config {
	config := new(Config)
	config.Version = version
	config.Path = path
	config.Args = args
	config.Pre = pre
	config.Env = env
	config.AutoRestart = autoRestart
	config.RestartTimes = restartTimes
	config.StartNow = startNow
	// 探针默认关闭，所有参数留零值，stater.go 中会用 cfg.ProbeCmd != "" 判断是否启用
	config.Port = port
	config.Password = password
	config.LogCapacity = logCapacity
	config.LogMaxLineBytes = logMaxLineBytes
	config.FileLogEnabled = fileLogEnabled
	config.LocalLogPath = localLogPath
	config.LocalLogLifeDay = localLogLifeDay
	return config
}

// ParseConfigWithProbe 是带探针参数的扩展构造函数
func ParseConfigWithProbe(version string, path string, args string, pre string, env string, autoRestart bool, restartTimes int, startNow bool,
	port int, password string, logCapacity int, logMaxLineBytes int, fileLogEnabled bool, localLogPath string, localLogLifeDay int,
	probeCmd string, probeInterval int, probeTimeout int, probeFailureLimit int) *Config {
	c := ParseConfig(version, path, args, pre, env, autoRestart, restartTimes, startNow,
		port, password, logCapacity, logMaxLineBytes, fileLogEnabled, localLogPath, localLogLifeDay)
	c.ProbeCmd = probeCmd
	c.ProbeInterval = probeInterval
	c.ProbeTimeout = probeTimeout
	c.ProbeFailureLimit = probeFailureLimit
	return c
}