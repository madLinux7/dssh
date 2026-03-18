BINARY  := dssh
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test clean compress release

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/dssh/

compress: build
	@which upx > /dev/null 2>&1 || (echo "upx not installed, skipping compression" && exit 0)
	upx --best $(BINARY) || echo "UPX compression failed, using uncompressed binary"

release: compress
	@echo "Built and compressed: $(BINARY)"
	@ls -lh $(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY) dssh-*