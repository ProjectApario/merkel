.PHONY: all build-cli build-app bin clean-app clean-cli clean

all: bin build-cli build-app

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
