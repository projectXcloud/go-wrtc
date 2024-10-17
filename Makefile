.PHONY: build

run r:
	go run ./cmd/go-wrtc/main.go

runt rt:
	go run ./cmd/go-wrtc/main.go -t

build b:
	mkdir -p ./build
	go build -o ./build/go-wrtc ./cmd/go-wrtc/main.go 

runweb rw:
	go run ./go-web-test/web.go

devconn c:
	docker exec -it -w /workspaces/go-wrtc go_devcontainer  /bin/bash

devconncomp cc:
	docker exec -it -w /workspaces/go-wrtc go_devcontainer_compose  /bin/bash