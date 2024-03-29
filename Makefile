PREFIX ?= /usr/local/bin

all: kopyaship kopyashipd

kopyaship:
	go build -ldflags '-s -w' ./cmd/kopyaship

kopyashipd:
	go build -ldflags '-s -w' ./cmd/kopyashipd

test:
	go test -v -buildmode=default -race ./...

test-coverage:
	go test -buildmode=default -coverprofile coverage.out -covermode=atomic ./...

install:
	mv kopyaship kopyashipd $(PREFIX)

.PHONY: kopyaship kopyashipd test test-coverage install
