BINARY := vpings
PKG := ./cmd/vpings
DIST := dist

.PHONY: build test clean cross

build:
	go build -o bin/$(BINARY) $(PKG)

test:
	go test ./...

clean:
	rm -rf bin $(DIST)

cross:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(DIST)/$(BINARY)_linux_amd64 $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(DIST)/$(BINARY)_linux_arm64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $(DIST)/$(BINARY)_darwin_amd64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $(DIST)/$(BINARY)_darwin_arm64 $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(DIST)/$(BINARY)_windows_amd64.exe $(PKG)
