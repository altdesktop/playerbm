.PHONY = test fmt build
.DEFAULT_GOAL := build

test:
	go test -v github.com/acrisci/playerbm/...

fmt:
	go fmt github.com/acrisci/playerbm/...

build:
	go build
