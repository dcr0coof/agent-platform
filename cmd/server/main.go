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
	"time"

	"github.com/dcr0coof/agent-platform/internal/config"
	"github.com/dcr0coof/agent-platform/internal/llm"
	"github.com/dcr0coof/agent-platform/internal/tool"
	"github.com/dcr0coof/agent-platform/internal/tool/builtin"
	"github.com/dcr0coof/agent-platform/internal/trip"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	listen := flag.String("listen", "127.0.0.1:8080", "本机工作台监听地址")
	dbPath := flag.String("db", "data/trips.db", "SQLite 文件；每个文件仅允许一个服务实例")
	webDir := flag.String("web-dir", "web/dist", "构建后的前端目录")
	configPath := flag.String("config", "configs/config.yaml", "模型配置")
	demo := flag.Bool("demo", false, "显式启用本地演示，不调用外部模型和工具")
	timeout := flag.Duration("timeout", 2*time.Minute, "每轮任务总超时")
	devOrigin := flag.String("dev-origin", "", "可选本机 Vite 开发源，例如 http://127.0.0.1:5173")
	flag.Parse()
	if *timeout <= 0 {
		return fmt.Errorf("timeout 必须大于零")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return fmt.Errorf("listen 必须是 host:port")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("此版本只允许绑定本机回环地址，公开部署需要额外身份认证")
	}
	var runner trip.Runner
	if *demo {
		runner = trip.DemoRunner{}
	} else {
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		if cfg.LLM.APIKey == "" {
			return fmt.Errorf("请设置 AGENT_LLM_API_KEY，或显式使用 -demo 体验本地演示")
		}
		reg := tool.NewRegistry()
		reg.Register(&builtin.Calculator{})
		reg.Register(&builtin.DateTime{})
		if cfg.Weather.APIKey != "" {
			reg.Register(builtin.NewWeather(cfg.Weather.APIKey, cfg.Weather.BaseURL))
		}
		runner = trip.AgentRunner{Client: llm.NewOpenAI(cfg.LLM.APIKey, cfg.LLM.BaseURL, cfg.LLM.Model, cfg.LLM.MaxTokens, cfg.LLM.Temperature), Tools: reg, MaxMessages: cfg.Memory.MaxMessages, MaxIterations: cfg.Agent.MaxIterations}
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	store, err := trip.OpenStore(*dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}
	defer store.Close()
	service := trip.NewService(store, runner, *timeout)
	defer service.Close()
	handler := (&trip.HTTP{Service: service, Demo: *demo, WebDir: *webDir, AllowedOrigin: *devOrigin}).Handler()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			service.Close()
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		case <-stopped:
		}
	}()
	defer close(stopped)
	mode := "真实模型"
	if *demo {
		mode = "本地演示 · 无外部请求"
	}
	log.Printf("行迹工作台 http://%s · %s", listener.Addr(), mode)
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
