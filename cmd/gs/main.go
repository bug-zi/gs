package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/repl"
	"github.com/bugzi/gs/internal/workspace"
)

func main() {
	root := flag.String("root", ".", "学习工作区根目录（默认当前目录及其上级）")
	flag.Parse()
	ws, err := workspace.Detect(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := config.Load(ws.Root, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败:", err)
		os.Exit(1)
	}
	if err := repl.New(ws, cfg, os.Stdout).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
