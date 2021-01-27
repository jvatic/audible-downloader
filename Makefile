EXECUTABLE=adl
WINDOWS_GUI=$(EXECUTABLE)_windows_amd64_gui.exe
WINDOWS_CLI=$(EXECUTABLE)_windows_amd64_cli.exe
LINUX_GUI=$(EXECUTABLE)_linux_amd64_gui
LINUX_CLI=$(EXECUTABLE)_linux_amd64_cli
DARWIN_GUI=$(EXECUTABLE)_darwin_amd64_gui
DARWIN_CLI=$(EXECUTABLE)_darwin_amd64_cli
VERSION=$(shell git describe --tags --always --long --dirty)
APP_NAME="Audible Downloader"

.PHONY: all test clean

all: build

build: windows_gui window_cli linux_gui linux_cli darwin_gui darwin_cli
	@echo version: $(VERSION)

windows_gui: $(WINDOWS_GUI)

windows_cli: $(WINDOWS_CLI)

linux_gui: $(LINUX_GUI)

linux_cli: $(LINUX_CLI)

darwin_gui: $(DARWIN_GUI)

darwin_cli: $(DARWIN_CLI)

$(WINDOWS_GUI):
	env GOOS=windows GOARCH=amd64 go build -i -v -o cli_$(WINDOWS) -ldflags="-s -w -X main.version=$(VERSION)" ./gui

$(WINDOWS_CLI):
	env GOOS=windows GOARCH=amd64 go build -i -v -o cli_$(WINDOWS) -ldflags="-s -w -X main.version=$(VERSION)" ./cli

$(LINUX_GUI):
	env GOOS=linux GOARCH=amd64 go build -i -v -o cli_$(LINUX) -ldflags="-s -w -X main.version=$(VERSION)" ./gui

$(LINUX_CLI):
	env GOOS=linux GOARCH=amd64 go build -i -v -o cli_$(LINUX) -ldflags="-s -w -X main.version=$(VERSION)" ./cli

$(DARWIN_GUI):
	env GOOS=darwin GOARCH=amd64 go build -i -v -o cli_$(DARWIN) -ldflags="-s -w -X main.version=$(VERSION)" ./gui

$(DARWIN_CLI):
	env GOOS=darwin GOARCH=amd64 go build -i -v -o cli_$(DARWIN) -ldflags="-s -w -X main.version=$(VERSION)" ./cli

clean: ## Remove previous build
	rm $(WINDOWS_GUI)
	rm $(WINDOWS_CLI)
	rm $(LINUX_GUI)
	rm $(LINUX_CLI)
	rm $(DARWIN_GUI)
	rm $(DARWIN_CLI)

help: ## Display available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
