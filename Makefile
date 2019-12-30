.PHONY = test fmt build
.DEFAULT_GOAL := build

test:
	go test -v github.com/altdesktop/playerbm/...

fmt:
	go fmt github.com/altdesktop/playerbm/...

build:
	go build
