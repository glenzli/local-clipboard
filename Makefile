BINARY=local-clipboard
NAS_HOST?=user@nas
NAS_DIR?=/path/to/local-clipboard

build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/$(BINARY)-$(shell go env GOOS)-$(shell go env GOARCH) .

dist-all:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-linux-amd64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w" -o dist/$(BINARY)-darwin-arm64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-windows-amd64.exe .

deploy: dist-all
	scp -O dist/$(BINARY)-linux-amd64 $(NAS_HOST):$(NAS_DIR)/local-clipboard-linux-amd64
	ssh $(NAS_HOST) "cd $(NAS_DIR) && docker compose up -d --build"

.PHONY: build dist-all deploy
