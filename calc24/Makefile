.PHONY: all clean test

BINARY=calc24

all: $(BINARY)

$(BINARY): main.go
	go build -ldflags "-X main.version=$$(git describe --tags --abbrev=0 2>/dev/null || echo dev) -X main.commit=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.buildTime=$$(date -u +'%Y-%m-%dT%H:%M:%SZ') -X main.tag=$$(git describe --tags --exact-match HEAD 2>/dev/null || echo none)" -o $(BINARY) .

test: $(BINARY)
	go test -v ./...

clean:
	rm -f $(BINARY)