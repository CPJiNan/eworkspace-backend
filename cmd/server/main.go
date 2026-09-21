package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eworkspace/internal/config"
	"eworkspace/internal/database"
	"eworkspace/internal/handler"
	"eworkspace/internal/logger"
	"eworkspace/internal/repository"
	"eworkspace/internal/router"
	"eworkspace/internal/service"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	healthcheck := flag.Bool("healthcheck", false, "探测服务是否已就绪，成功退出码为 0")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("加载配置失败", "err", err, "path", *configPath)
		os.Exit(1)
	}

	if *healthcheck {
		os.Exit(probeHealth(cfg.Server.Addr()))
	}

	log := logger.New(cfg.Log)
	slog.SetDefault(log)

	db, err := database.Open(cfg.Database)
	if err != nil {
		log.Error("初始化数据库失败", "err", err)
		os.Exit(1)
	}
	if err := database.Migrate(db); err != nil {
		log.Error("数据库迁移失败", "err", err)
		os.Exit(1)
	}
	if err := database.Seed(db, cfg.Bootstrap, log); err != nil {
		log.Error("初始化内置数据失败", "err", err)
		os.Exit(1)
	}

	repos := repository.New(db)
	svcs := service.New(cfg, repos, log)
	handlers := handler.New(svcs, log)

	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go svcs.DeadlineTimer.Run(workerCtx)

	engine := router.New(cfg, log, svcs, handlers)

	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("EWorkspace 后端启动",
			"version", version,
			"addr", srv.Addr,
			"db", cfg.Database.Path,
			"env", cfg.Server.Mode,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP 服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("收到退出信号，开始关闭")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("关闭失败", "err", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	log.Info("服务已退出")
}

func probeHealth(addr string) int {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "健康检查失败：无法连接 %s：%v\n", addr, err)
		return 1
	}
	_ = conn.Close()
	return 0
}
