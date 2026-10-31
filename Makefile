.PHONY: help run dev build package start test test-e2e server clean asr android-test android-build android-install install-apk install apk livekit-check livekit-stop

JAVA_HOME ?= $(shell if [ -d "/Applications/Android Studio.app/Contents/jbr/Contents/Home" ]; then echo "/Applications/Android Studio.app/Contents/jbr/Contents/Home"; fi)
ANDROID_HOME ?= $(shell if [ -d "/Users/fx/Library/Android/sdk" ]; then echo "/Users/fx/Library/Android/sdk"; fi)
GRADLEW = JAVA_HOME="$(JAVA_HOME)" ANDROID_HOME="$(ANDROID_HOME)" ./gradlew
# 配置变量
TAGS ?= production
LDFLAGS ?= -w -s

# 默认目标
help: ## 显示帮助信息
	@echo "Avatar 构建与管理指令:"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

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

core-build: ## 编译跨平台 C++ 核心库 (libaudio_agent.dylib / libaudio_agent.a)
	@echo "==> 构建 core C++ 原生跨平台通用库..."
	cmake -B core/build -S core && cmake --build core/build

native-test: ## 编译并在宿主机运行 C++ 核心单元测试套件
	@echo "==> 运行 C++17 核心单元冒烟测试套件..."
	cd core && clang++ -std=c++17 -O2 -Wall -Wextra \
		-I include \
		src/CpuAffinity.cpp \
		src/IntentArbiter.cpp \
		src/StreamingSentenceChunker.cpp \
		src/WatchdogAndRepetition.cpp \
		src/ThreeTierMemory.cpp \
		src/FullDuplexAudioIO.cpp \
		src/AudioAgentPipeline.cpp \
		src/MnnLlmEngine.cpp \
		src/SherpaOnnxPerceptionEngine.cpp \
		src/audio_agent_c_api.cpp \
		src/test_main.cpp \
		-o audio_agent_test && ./audio_agent_test && rm -f audio_agent_test

native-bench: ## 运行 C++17 全双工性能基准测试
	@echo "==> 运行 C++17 性能基准测试..."
	cd core && clang++ -std=c++17 -O3 \
		-I include \
		src/CpuAffinity.cpp \
		src/IntentArbiter.cpp \
		src/StreamingSentenceChunker.cpp \
		src/WatchdogAndRepetition.cpp \
		src/ThreeTierMemory.cpp \
		src/FullDuplexAudioIO.cpp \
		src/AudioAgentPipeline.cpp \
		src/MnnLlmEngine.cpp \
		src/SherpaOnnxPerceptionEngine.cpp \
		src/audio_agent_c_api.cpp \
		src/bench_main.cpp \
		-o audio_agent_bench && ./audio_agent_bench && rm -f audio_agent_bench
android-test: ## 运行 Android 客户端单元测试
	cd android && $(GRADLEW) test

android-build: ## 构建 Android 客户端 Debug APK
	cd android && $(GRADLEW) assembleDebug

apk: android-build ## 构建 Android 客户端 Debug APK (android-build 的别名)

android-install: ## 构建并安装 Android Debug APK 到已连接的设备或模拟器
	cd android && ./gradlew installDebug

install-apk: android-install ## 安装 Android Debug APK 到设备 (android-install 的别名)

install: android-install ## 安装应用到设备 (默认安装 Android APK)

livekit-check:
	@if nc -z -w 1 localhost 7880 2>/dev/null; then \
		echo "\033[32m[LiveKit]\033[0m SFU 已在运行 (:7880)"; \
	elif docker info >/dev/null 2>&1; then \
		echo "\033[33m[LiveKit]\033[0m 正在后台自动拉起 LiveKit SFU 容器 (:7880)..."; \
		docker compose -f server/docker-compose.yaml up -d && \
		echo "\033[32m[LiveKit]\033[0m SFU 容器已就绪 (:7880)"; \
	else \
		echo "\033[33m[LiveKit 提示]\033[0m Docker 守护进程未启动，跳过自动拉起。云端网关继续启动 (:8088)，如需使用 WebRTC 流媒体请先启动 Docker"; \
	fi

server: livekit-check ## 启动本地轻量云端实时网关服务 (8088 端口，自动拉起 LiveKit SFU)
	go run ./server/cmd/main.go

livekit-stop: ## 停止后台运行的 LiveKit SFU 容器
	@docker compose -f server/docker-compose.yaml down

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
