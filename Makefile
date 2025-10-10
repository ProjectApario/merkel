.PHONY: all build-cli build-app bin clean-app clean-cli clean install

all: bin build-cli build-app install

bin:
	[ ! -d bin ] && mkdir -p bin

clean: bin clean-app clean-cli

clean-app:
	@[ -f bin/merkel-app ] && rm -rf bin/merkel-app

clean-cli:
	@if [ -f bin/merkel-cli ]; then rm -rf bin/merkel-cli; fi;

build-app: clean-app
	@go build -o bin/merkel-app -ldflags="-s -w" ./app/main.go

build-cli: clean-cli
	go build -o bin/merkel-cli -ldflags="-s -w" ./cli/main.go

install:
	@if [ -d "${HOME}/bin" ] && [ -f "${HOME}/bin/merkel-cli" ]; then rm -rf "${HOME}/bin/merkel-cli"; fi
	@if [ -d bin ] && [ -f bin/merkel-cli ]; then chmod +x bin/merkel-cli && cp bin/merkel-cli "${HOME}/bin/merkel-cli"; fi
