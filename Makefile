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

runhaproxy ha:
	haproxy -f /etc/haproxy/haproxy.cfg

playfile pf:
	ffplay -nodisp -loop -1 /workspaces/go-wrtc/file2.mp3

devconn c:
	docker exec -it -w /workspaces/go-wrtc go_devcontainer  /bin/bash

devconncomp cc:
	docker exec -it -w /workspaces/go-wrtc go_devcontainer_compose  /bin/bash

openvscodedevconncomp vscc:
	code --folder-uri "vscode-remote://attached-container+$(printf "{\"containerName\":\"/go_devcontainer_compose\"}" | od -A n -t x1 | tr -d '[\n\t ]')/workspaces/go-wrtc"
