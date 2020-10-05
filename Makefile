GIT_VER:=$(shell git describe --tags)
DATE:=$(shell date +%Y-%m-%dT%H:%M:%SZ)
export GO111MODULE := on

.PHONY: test get-deps install clean

all: test build

install:
	cd cmd/sqsjkr && go build -ldflags "-X main.version=${GIT_VER} -X main.buildDate=${DATE}"
	install cmd/sqsjkr/sqsjkr ${GOPATH}/bin

test:
	go test . -v

clean:
	rm -f cmd/sqsjkr/sqsjkr
	rm -f pkg/*
