package core

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/global"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// 读取配置

func Viper() *viper.Viper {

	cfg_path := GetConfigPath()
	v := viper.New()
	v.SetConfigFile(cfg_path)
	v.SetConfigType("toml")
	v.SetEnvPrefix("VISTACK")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := v.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
	v.WatchConfig()

	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("config file changed:", e.Name)
	})
	if err = v.Unmarshal(&global.AppConfig); err != nil {
		panic(fmt.Errorf("fatal error unmarshal config: %w", err))
	}
	return v
}

func GetConfigPath() (cfg string) {
	flag.StringVar(&cfg, "c", "", "set config file")
	flag.Parse()
	if cfg != "" { // 命令行参数不为空 将值赋值于config
		fmt.Printf("您正在使用命令行的 '-c' 参数传递的值, config 的路径为 %s\n", cfg)
		return
	}
	if os.Getenv(config.VISTACK_CONFIG_PATH) != "" { // 判断环境变量 VISTACK_CONFIG_PATH
		cfg = os.Getenv(config.VISTACK_CONFIG_PATH)
		fmt.Printf("您正在使用 %s 环境变量, config 的路径为 %s\n", config.VISTACK_CONFIG_PATH, cfg)
		return
	}
	if cfg == "" { // 命令行参数和环境变量都为空, 使用默认值
		cfg = config.DefaultConfigPath
		fmt.Printf("您正在使用默认值, config 的路径为 %s\n", cfg)
		return
	}
	return
}
