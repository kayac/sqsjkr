GIT_VER:=$(shell git describe --tags)
DATE:=$(shell date +%Y-%m-%dT%H:%M:%SZ)
export GO111MODULE := on

.PHONY: test get-deps install clean

all: test build

install:
	cd cmd/sqsjkr && go build -ldflags "-X main.version=${GIT_VER} -X main.buildDate=${DATE}"
	install cmd/sqsjkr/sqsjkr ${GOPATH}/bin

packages:
	cd cmd/sqsjkr && gox -os="linux darwin" -arch="amd64" -output "../../pkg/{{.Dir}}-${GIT_VER}-{{.OS}}-{{.Arch}}" -gcflags "-trimpath=${GOPATH}" -ldflags "-w -s -X main.version=${GIT_VER} -X main.buildDate=${DATE}"
	cd pkg && find . -name "*${GIT_VER}*" -type f -exec tar czf {}.tar.gz {} \;

test:
	go test . -v

clean:
	rm -f cmd/sqsjkr/sqsjkr
	rm -f pkg/*

build:
	cd cmd/sqsjkr && go build -o sqsjkr -gcflags="-trimpath=${HOME}" -ldflags="-w -s -X main.version=${GIT_VER} -X main.buildDate=${DATE}"
