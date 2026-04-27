package main

import (
	"log"

	"myclouddrive-go/internal/app"
	filemodule "myclouddrive-go/internal/file/module"
	pluginmodule "myclouddrive-go/internal/plugin/module"
	sharemodule "myclouddrive-go/internal/share/module"
	storagemodule "myclouddrive-go/internal/storage/module"
	usermodule "myclouddrive-go/internal/user/module"
)

const configPath = "configs/config.yaml"

// OpenAPI + net/http + GORM + Redis 工程化入口（模块化装配版）。
func main() {
	srv, err := app.NewServer(
		configPath,
		pluginmodule.New(),
		storagemodule.New(),
		usermodule.New(),
		filemodule.New(),
		sharemodule.New(),
	)
	if err != nil {
		log.Fatalf("build server failed: %v", err)
	}
	if err = srv.Run(); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
}
