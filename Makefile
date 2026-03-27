VERSION := $(shell git describe --tags --always --dirty)
LDFLAGS := -ldflags="-X github.com/dyegoe/awss/cmd.version=$(VERSION)"

.PHONY: build test lint clean

build:
	go build $(LDFLAGS) -o awss .

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f awss
