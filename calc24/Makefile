.PHONY: all clean test

BINARY=calc24

all: $(BINARY)

$(BINARY): main.go
	go build -ldflags "-X main.version=$$(git describe --tags --abbrev=0 2>/dev/null || echo dev) -X main.commit=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.buildTime=$$(date -u +'%Y-%m-%dT%H:%M:%SZ')" -o $(BINARY) .

test: $(BINARY)
	go test ./...
	./$(BINARY) 1 3 6 9
	./$(BINARY) 3 3 7 7
	./$(BINARY) 7 7 7 7

clean:
	rm -f $(BINARY)