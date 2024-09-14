.PHONY: build

run:
	go run ./cmd/go-wrtc/main.go

build:
	mkdir -p ./build
	go build -o ./build/go-wrtc ./cmd/go-wrtc/main.go 

runweb:
	go run ./go-web-tbdl/web.go