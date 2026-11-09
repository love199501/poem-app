package main

import (
	"log"

	"shijing/internal/config"
	"shijing/internal/db"
	"shijing/internal/handler"
	"shijing/internal/router"
	"shijing/internal/seed"
)

func main() {
	cfg := config.Load()

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
	r := router.Setup(h)

	addr := ":" + cfg.AppPort
	log.Printf("[server] listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
