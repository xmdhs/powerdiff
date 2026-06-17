package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xmdhs/powerdiff/internal/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "HTTP 监听地址")
	noBrowser := flag.Bool("no-browser", false, "不自动打开浏览器")
	debug := flag.Bool("debug", false, "输出调试日志")
	flag.Parse()

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("监听失败: %v", err)
	}

	app := server.New(server.Config{Debug: *debug})
	httpServer := &http.Server{Handler: app.Handler(), ReadHeaderTimeout: 10 * time.Second}

	url := fmt.Sprintf("http://%s/", listener.Addr().String())
	fmt.Printf("Power Web UI 正在运行: %s\n", url)
	fmt.Println("按 Ctrl+C 停止。")

	if !*noBrowser {
		go func() {
			if err := server.OpenBrowser(url); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n请手动打开: %s\n", err, url)
			}
		}()
	}

	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务失败: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("关闭服务失败: %v", err)
	}
}
