# puzzle 构建脚本
#
# 常用目标：
#   make / make build   构建 Linux (WSL) 可执行文件
#   make windows        构建 Windows 可执行文件（从 Linux 交叉编译）
#   make all            构建 Linux + Windows 两个平台
#   make test           运行单元测试
#   make run            编译并直接运行
#   make clean          清理构建产物

APP    := puzzle
DIST   := dist
GO     ?= go
GOARCH ?= amd64

LINUX_BIN   := $(DIST)/$(APP)-linux-$(GOARCH)
WINDOWS_BIN := $(DIST)/$(APP)-windows-$(GOARCH).exe

GOFILES := $(wildcard *.go) go.mod go.sum

# 本机（WSL）缺少 GL/X11 的 C 开发头文件，Linux 目标使用 ebiten 纯 Go
# 编译模式（purego），运行时 dlopen WSLg 自带的系统库。
# 若已安装 libgl1-mesa-dev xorg-dev libasound2-dev，可去掉 purego 标签。
LINUX_FLAGS   := -tags purego -trimpath -ldflags "-s -w"
# -H=windowsgui：双击运行时不弹出控制台窗口；调试期若需查看 log 输出，
# 可临时去掉该选项。
WINDOWS_FLAGS := -trimpath -ldflags "-s -w -H=windowsgui"

.DEFAULT_GOAL := build
.PHONY: all build windows test run clean

all: build windows

build: $(LINUX_BIN)

windows: $(WINDOWS_BIN)

$(LINUX_BIN): $(GOFILES)
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=$(GOARCH) $(GO) build $(LINUX_FLAGS) -o $@ .

$(WINDOWS_BIN): $(GOFILES)
	@mkdir -p $(DIST)
	GOOS=windows GOARCH=$(GOARCH) $(GO) build $(WINDOWS_FLAGS) -o $@ .

test:
	$(GO) test -tags purego -count=1 ./...

run:
	$(GO) run -tags purego .

clean:
	rm -rf $(DIST) $(APP)
