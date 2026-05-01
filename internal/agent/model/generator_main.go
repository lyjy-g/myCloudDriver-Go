//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

const (
	defaultDSN = "myclouddrive:myclouddrive@tcp(127.0.0.1:3306)/myclouddrive?charset=utf8mb4&parseTime=True&loc=Local"
	queryPath  = "./gen"
	modelPkg   = "dbmodel"
)

func main() {
	dsn := os.Getenv("MODEL_GEN_DSN")
	if dsn == "" {
		dsn = defaultDSN
		log.Printf("MODEL_GEN_DSN 未设置，使用默认 DSN: %s", defaultDSN)
	}

	cfg := gen.Config{
		OutPath:      queryPath,
		Mode:         gen.WithDefaultQuery,
		ModelPkgPath: modelPkg,
	}

	g := gen.NewGenerator(cfg)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	g.UseDB(db)

	tables := []string{
		"agent_audit_log",
		"agent_action",
		"agent_action_step",
		"agent_prompt_template",
		"agent_tool",
		"agent_tool_call",
		"knowledge",
		"knowledge_file",
		"knowledge_document_chunk",
		"workflow_definition",
		"workflow_run",
		"workflow_node_run",
	}
	models := make([]any, 0, len(tables))
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			log.Fatalf("表不存在: %s", table)
		}
		models = append(models, g.GenerateModel(table))
	}

	g.ApplyBasic(models...)
	g.Execute()

	fmt.Printf("Agent 模型生成完成：query=%s modelPkg=%s\n", queryPath, modelPkg)
}
