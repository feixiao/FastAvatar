.PHONY: help run dev build package start test clean asr

# 配置变量
TAGS ?= production
LDFLAGS ?= -w -s

# 默认目标
help: ## 显示帮助信息
	@echo "Avatar 构建与管理指令:"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

asr: ## 编译 macOS 原生语音识别组件 (bin/avatar-asr)
	@mkdir -p bin
	swiftc -O -framework Speech -framework AVFoundation cmd/avatar-asr/main.swift -o bin/avatar-asr

run: asr ## 直接运行应用 (开发调试)
	go run -tags $(TAGS) .

dev: run ## run 的别名

test: ## 运行所有单元测试
	go test -v ./...

test-e2e: ## 运行端云协同端到端集成对接与打断贯通测试
	go run ./cmd/e2e-test/main.go

server: ## 启动本地轻量云端实时网关服务 (8088 端口)
	go run ./server/cmd/main.go

build: asr ## 编译可执行二进制文件
	@mkdir -p build/bin
	go build -tags $(TAGS) -ldflags="$(LDFLAGS)" -o build/bin/Avatar .
package: ## 打包为 macOS 原生应用 (build/bin/Avatar.app)
	@chmod +x ./package_mac.sh
	./package_mac.sh

start: package ## 打包并直接启动 macOS App
	open build/bin/Avatar.app

clean: ## 清理构建产物与临时文件
	rm -rf build/bin
