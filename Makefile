VERSION=0.0.6
LDFLAGS=-ldflags "-X main.Version=${VERSION}"
all: motarei

.PHONY: motarei

bundle:
	dep ensure

update:
	dep ensure -update

motarei: motarei.go
	go build $(LDFLAGS) -o motarei

linux: motarei.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o motarei

fmt:
	go fmt ./...

clean:
	rm -rf motarei

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin master
	goreleaser --rm-dist
