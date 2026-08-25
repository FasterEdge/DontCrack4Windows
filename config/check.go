package config

import (
	"errors"
	"fmt"
	"os"
)

// 检查配置的合法性
func CheckConfig(config Config) error {
	if config.Version == "" {
		fmt.Println("系统版本号为空，跳过配置检查")
		return nil
	}
	if config.Path == "" {
		return fmt.Errorf("所管理进程的Path不可以为空")
	}
	if !fileExists(config.Path) {
		return fmt.Errorf("所管理进程的Path指定的文件不存在: %s", config.Path)
	}
	return nil
}

// 检查指定位置的文件存在并且是文件而不是文件夹
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		return false
	}
	return !info.IsDir()
}