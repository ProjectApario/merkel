.PHONY: all build-cli build-app bin clean-app clean-cli clean install uninstall build

all: clean build install

bin:
	@if [ ! -d bin ]; then mkdir -p bin; fi
	@echo "Installed bin directory!"

clean: clean-app clean-cli

clean-app: bin
	@if [ -f bin/merkel-app ]; then rm -rf bin/merkel-app; fi;
	@echo "Cleaned bin/merkel-app"

clean-cli: bin
	@if [ -f bin/merkel-cli ]; then rm -rf bin/merkel-cli; fi;
	@echo "Cleaned bin/merkel-cli"

build: bin build-app build-cli

build-app: clean-app
	@go build -o bin/merkel-app -ldflags="-s -w" ./app/main.go
	@echo "Compiled bin/merkel-app"

build-cli: clean-cli
	@go build -o bin/merkel-cli -ldflags="-s -w" ./cli/main.go
	@echo "Compiled bin/merkel-cli"

install:
	@if [ -d "${HOME}/bin" ] && [ -f "${HOME}/bin/merkel-cli" ]; then rm -rf "${HOME}/bin/merkel-cli"; fi
	@if [ -d bin ] && [ -f bin/merkel-cli ]; then chmod +x bin/merkel-cli && cp bin/merkel-cli "${HOME}/bin/merkel-cli"; fi
	@echo "Installed ~/bin/merkel-cli"

uninstall:
	@if [ -d "${HOME}/bin" ] && [ -f "${HOME}/bin/merkel-cli" ]; then rm -rf "${HOME}/bin/merkel-cli"; fi
	@echo "Removed ~/bin/merkel-cli"
