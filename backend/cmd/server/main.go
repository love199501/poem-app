package main

import (
	"log"
	_ "time/tzdata"

	"shijing/internal/config"
	"shijing/internal/db"
	"shijing/internal/handler"
	"shijing/internal/router"
	"shijing/internal/seed"
)

func main() {
	cfg := config.Load()

	// 打印实际使用的数据库连接目标（不含密码），便于定位云托管 1045/连接问题。
	log.Printf("[db] 连接目标 user=%s host=%s port=%s db=%s", cfg.DBUser, cfg.DBHost, cfg.DBPort, cfg.DBName)

	gdb, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("connect db failed: %v", err)
	}

	if cfg.AutoMigrate {
		if err := seed.Migrate(gdb); err != nil {
			log.Fatalf("migrate failed: %v", err)
		}
		if err := seed.EnsureAdmin(gdb); err != nil {
			log.Printf("ensure admin failed: %v", err)
		}
		if err := seed.SyncCounts(gdb); err != nil {
			log.Printf("sync counts failed: %v", err)
		}
	}
	if cfg.Seed {
		if err := seed.Run(gdb); err != nil {
			log.Fatalf("seed failed: %v", err)
		}
	}

	h := handler.New(gdb, cfg)
	h.StartIllustBackfill()
	r := router.Setup(h)

	addr := ":" + cfg.AppPort
	log.Printf("[server] listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
