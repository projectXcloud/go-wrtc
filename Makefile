.PHONY: build

run:
	go run ./cmd/go-wrtc/main.go

runt:
	go run ./cmd/go-wrtc/main.go -t

build:
	mkdir -p ./build
	go build -o ./build/go-wrtc ./cmd/go-wrtc/main.go 

runweb:
	go run ./go-web-test/web.go

devconn:
	docker exec -it -w /workspaces/go-wrtc go_devcontainer  /bin/bash