package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	"github.com/binhy/vistack/internal/role"
)

func main() {
	roleName := resolveRole()

	core.Viper() // 初始化 Viper 并解析配置
	cfg := global.AppConfig

	core.InitLogger(&cfg)
	defer core.SyncLogger()

	switch roleName {
	case "api":
		role.RunAPI(&cfg)
	case "worker":
		role.RunWorker(&cfg)
	case "transcoder":
		role.RunTranscoder(&cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown role %q (expected api|worker|transcoder)\n", roleName)
		os.Exit(1)
	}
}

// resolveRole 解析角色：优先 VISTACK_ROLE 环境变量，其次第一个非 flag 位置参数，缺省 api
func resolveRole() string {
	if r := os.Getenv("VISTACK_ROLE"); r != "" {
		return r
	}
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		return os.Args[1]
	}
	return "api"
}
