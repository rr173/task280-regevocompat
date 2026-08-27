// Command regevocompat 是「跨区域数据库模式演进兼容验证服务」的可执行入口。
//
// 用法：
//
//	regevocompat --smoke-test                运行内置自检（建库、冲突定位、兼容窗口消解、快照发布、重启恢复）
//	regevocompat --addr :8080 --db ./regevocompat.db   启动 HTTP API 服务
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"task280-regevocompat/internal/httpapi"
	"task280-regevocompat/internal/service"
	"task280-regevocompat/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "./regevocompat.db", "SQLite 数据库文件路径")
	smoke := flag.Bool("smoke-test", false, "运行内置自检并退出（不启动服务）")
	flag.Parse()

	if *smoke {
		if err := service.SmokeTest(*dbPath); err != nil {
			log.Fatalf("smoke-test failed: %v", err)
		}
		return
	}

	s, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	svc := service.New(s)
	h := httpapi.New(svc)
	mux := http.NewServeMux()
	h.Routes(mux)

	fmt.Printf("regevocompat listening on %s (db=%s)\n", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
