package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var moduleGenerators = map[string]string{
	"user":    "internal/user/model/generator_main.go",
	"storage": "internal/storage/model/generator_main.go",
	"share":   "internal/share/model/generator_main.go",
	"file":    "internal/file/model/generator_main.go",
	"agent":   "internal/agent/model/generator_main.go",
}

func main() {
	modules := selectedModules(os.Args[1:])
	for _, module := range modules {
		if err := runModuleGenerator(module, moduleGenerators[module]); err != nil {
			log.Fatalf("生成模块 %s 模型失败: %v", module, err)
		}
	}

	fmt.Printf("模型生成完成：modules=%s\n", strings.Join(modules, ","))
}

func selectedModules(args []string) []string {
	if len(args) == 0 {
		return []string{"user", "storage", "share", "file", "agent"}
	}

	modules := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	for _, arg := range args {
		name := strings.TrimSpace(arg)
		if name == "" {
			continue
		}
		if _, ok := moduleGenerators[name]; !ok {
			log.Fatalf("未知模块 %q，可选模块: %s", name, strings.Join(allModuleNames(), ","))
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		modules = append(modules, name)
	}
	if len(modules) == 0 {
		log.Fatalf("未提供有效模块，可选模块: %s", strings.Join(allModuleNames(), ","))
	}
	return modules
}

func runModuleGenerator(module string, generatorPath string) error {
	absPath, err := filepath.Abs(generatorPath)
	if err != nil {
		return fmt.Errorf("解析路径失败: %w", err)
	}

	// 统一入口只负责调度，各业务模块仍维护自己的表清单和生成逻辑。
	cmd := exec.Command("go", "run", absPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Dir(absPath)

	log.Printf("开始生成模块模型: module=%s path=%s", module, generatorPath)
	if err := cmd.Run(); err != nil {
		return err
	}
	log.Printf("模块模型生成完成: module=%s", module)
	return nil
}

func allModuleNames() []string {
	return []string{"user", "storage", "share", "file", "agent"}
}
