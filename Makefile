APP        := msqlite
GOFLAGS    := -trimpath
VERSION_SH := ./scripts/version.sh

VERSION    := $(shell $(VERSION_SH) version)
COMMIT     := $(shell $(VERSION_SH) commit)
BUILD_DATE := $(shell $(VERSION_SH) date)
LDFLAGS    := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: all build pi pi32 dist publish release ship test vet clean install uninstall version

all: build

build:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(APP) .

pi:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(APP)-linux-arm64 .

pi32:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(APP)-linux-armv7 .

dist: pi pi32 publish

publish: build pi
	./scripts/publish-adeploy.sh

release: dist

ship:
	./scripts/ship.sh

test:
	go test ./...

vet:
	go vet ./...

install:
	./install.sh

uninstall:
	./install.sh --uninstall

version:
	@$(VERSION_SH) full

clean:
	rm -rf bin
