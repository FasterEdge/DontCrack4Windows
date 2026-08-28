package config

import (
	"os"
	"strings"
	"testing"
)

func TestCheckConfigRejectsInvalidNumericParams(t *testing.T) {
	base := Config{
		Version:         "test",
		Path:            os.Args[0], // 测试二进制自身一定存在且是文件
		Port:            11883,
		LogCapacity:     200,
		LogMaxLineBytes: 1048576,
		RestartTimes:    3,
		LocalLogLifeDay: 7,
	}
	if err := CheckConfig(base); err != nil {
		t.Fatalf("baseline config should pass, got: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(*Config)
		contain string
	}{
		{"negative log capacity", func(c *Config) { c.LogCapacity = -1 }, "log-capacity"},
		{"zero log max line bytes", func(c *Config) { c.LogMaxLineBytes = 0 }, "log-max-line-bytes"},
		{"negative log max line bytes", func(c *Config) { c.LogMaxLineBytes = -1 }, "log-max-line-bytes"},
		{"port too low", func(c *Config) { c.Port = 0 }, "port"},
		{"port too high", func(c *Config) { c.Port = 70000 }, "port"},
		{"invalid retry count", func(c *Config) { c.RestartTimes = -2 }, "max-retries"},
		{"negative log life day", func(c *Config) { c.LocalLogLifeDay = -1 }, "log-life-day"},
		{"probe without interval", func(c *Config) { c.ProbeCmd = "true"; c.ProbeInterval = 0; c.ProbeTimeout = 5; c.ProbeFailureLimit = 3 }, "probe-interval"},
		{"probe without timeout", func(c *Config) { c.ProbeCmd = "true"; c.ProbeTimeout = 0; c.ProbeInterval = 30; c.ProbeFailureLimit = 3 }, "probe-timeout"},
		{"probe without failure limit", func(c *Config) { c.ProbeCmd = "true"; c.ProbeFailureLimit = 0; c.ProbeInterval = 30; c.ProbeTimeout = 5 }, "probe-failure-limit"},
		{"missing path", func(c *Config) { c.Path = "" }, "Path"},
		{"nonexistent path", func(c *Config) { c.Path = "/definitely/not/exists/xyz" }, "不存在"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base // 每个用例独立拷贝，避免前序用例的 ProbeCmd 泄漏
			tc.mutate(&cfg)
			err := CheckConfig(cfg)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.contain) {
				t.Fatalf("error %q should mention %q", err.Error(), tc.contain)
			}
		})
	}
}

func TestCheckConfigSkippedWithoutVersion(t *testing.T) {
	// Version 为空时跳过检查（兼容旧行为）
	err := CheckConfig(Config{})
	if err != nil {
		t.Fatalf("empty version should skip checks, got: %v", err)
	}
}

func TestParseConfigListenDefaults(t *testing.T) {
	cfg := ParseConfigListen("v", "/bin/true", "", "", "", false, 3, false, 11883, "", "", 200, 1048576, false, "", 7)
	if cfg.ListenAddress != "127.0.0.1" {
		t.Fatalf("empty listen address should default to 127.0.0.1, got %q", cfg.ListenAddress)
	}
}
