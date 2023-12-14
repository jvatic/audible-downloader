EXECUTABLE=adl
WINDOWS_GUI=$(EXECUTABLE)_windows_amd64_gui
WINDOWS_CLI=$(EXECUTABLE)_windows_amd64_cli.exe
LINUX_CLI=$(EXECUTABLE)_linux_amd64_cli
DARWIN_GUI=$(EXECUTABLE)_darwin_amd64_gui
DARWIN_CLI=$(EXECUTABLE)_darwin_amd64_cli
VERSION=$(shell git describe --tags --always --long --dirty)
APP_NAME="Audible Downloader"

.PHONY: all test clean

all: build

build: windows_cli linux_cli darwin_gui darwin_cli ## Build all targets except GUI for Windows
	@echo version: $(VERSION)

build_all: build windows_gui ## Build all targets including GUI for Windows (requires xgo)

windows_gui: $(WINDOWS_GUI) ## Build GUI for Windows

windows_cli: $(WINDOWS_CLI) ## Build CLI for Windows

linux_cli: $(LINUX_CLI) ## Build CLI for Linux

darwin_gui: $(DARWIN_GUI) ## Build GUI for Darwin (macOS)

darwin_cli: $(DARWIN_CLI) ## Build CLI for Darwin (macOS)

$(WINDOWS_GUI):
	xgo -targets windows/amd64 -out adl_windows_amd64_gui -ldflags="-s -w -X main.version=$(VERSION)" ./gui
	mv $(shell find $(WINDOWS_GUI)-*.exe) $(WINDOWS_GUI).exe

$(WINDOWS_CLI):
	env GOOS=windows GOARCH=amd64 go build -v -o $(WINDOWS_CLI) -ldflags="-s -w -X main.version=$(VERSION)" ./cli

$(LINUX_CLI):
	env GOOS=linux GOARCH=amd64 go build -v -o $(LINUX_CLI) -ldflags="-s -w -X main.version=$(VERSION)" ./cli

$(DARWIN_GUI):
	env GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -v -o $(DARWIN_GUI) -ldflags="-s -w -X main.version=$(VERSION)" ./gui

$(DARWIN_CLI):
	env GOOS=darwin GOARCH=amd64 go build -v -o $(DARWIN_CLI) -ldflags="-s -w -X main.version=$(VERSION)" ./cli

clean: ## Remove previous build
	rm -f $(WINDOWS_GUI).exe
	rm -f $(WINDOWS_CLI)
	rm -f $(LINUX_CLI)
	rm -f $(DARWIN_GUI)
	rm -f $(DARWIN_CLI)

help: ## Display available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
