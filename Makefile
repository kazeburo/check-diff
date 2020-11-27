VERSION=0.0.3
LDFLAGS=-ldflags "-w -s -X main.Version=${VERSION}"

all: check-diff

.PHONY: check-diff

check-diff: main.go
	go build $(LDFLAGS) -o check-diff main.go

linux: main.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o check-diff main.go

check:
	go test ./...

fmt:
	go fmt ./...

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin master
