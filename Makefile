BINARY=local-clipboard
UNAME_S=$(shell uname -s)
UNAME_M=$(shell uname -m)

-include deploy.env

ifeq ($(UNAME_S),Darwin)
  ifeq ($(UNAME_M),arm64)
    LOCAL_BIN=local-clipboard-darwin-arm64
  else
    LOCAL_BIN=local-clipboard-darwin-amd64
  endif
else ifeq ($(UNAME_S),Linux)
  LOCAL_BIN=local-clipboard-linux-amd64
endif

# darwin keeps cgo enabled: the tray icon (systray) needs it.
build:
	go build -ldflags "-s -w" -o dist/$(BINARY)-$(shell go env GOOS)-$(shell go env GOARCH) .

dist-all:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-linux-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-windows-amd64.exe .

deploy: dist-all
	@test -n "$(NAS_HOST)" || { echo "missing NAS_HOST: cp deploy.env.example deploy.env"; exit 1; }
	test -n "$(NAS_DIR)"  || { echo "missing NAS_DIR"; exit 1; }
	scp -O dist/$(BINARY)-linux-amd64 "$(NAS_HOST):$(NAS_DIR)/local-clipboard-linux-amd64"
	ssh "$(NAS_HOST)" "cd '$(NAS_DIR)' && docker compose up -d --build"

install:
ifndef LOCAL_BIN
	$(error unsupported local OS $(UNAME_S)/$(UNAME_M))
endif
	test -n "$(LOCALCLIP_SERVER)" || { echo "missing LOCALCLIP_SERVER: cp deploy.env.example deploy.env"; exit 1; }
	$(MAKE) build
	@if [ "$(UNAME_S)" = "Darwin" ]; then \
	  cp dist/$(LOCAL_BIN) deploy/macos/; \
	  (cd deploy/macos && LOCALCLIP_SERVER=$(LOCALCLIP_SERVER) LOCALCLIP_TOKEN=$(LOCALCLIP_TOKEN) ./install.sh); \
	else \
	  cp dist/$(LOCAL_BIN) deploy/linux/; \
	  (cd deploy/linux && LOCALCLIP_SERVER=$(LOCALCLIP_SERVER) LOCALCLIP_TOKEN=$(LOCALCLIP_TOKEN) ./install.sh); \
	fi

rollout: deploy install
	@echo ""
	@echo "done. NAS hub upgraded + this machine's daemon reinstalled."
	@echo "UI: http://127.0.0.1:9101"

.PHONY: build dist-all deploy install rollout
