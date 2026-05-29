SHELL := /bin/bash
TARGETS := webshare fixfn bytehist

.PHONY: all
all: $(TARGETS)

%: cmd/%/main.go
	go build -ldflags="-s -w" -o $@ $<

.PHONY: clean
clean:
	rm -f $(TARGETS)
	rm -f *.deb
	rm -f *.rpm

.PHONY: deb
deb: $(TARGETS)
	GOARCH=amd64 SEMVER=$(VERSION) nfpm package -p deb
	GOARCH=arm64 SEMVER=$(VERSION) nfpm package -p deb

.PHONY: rpm
rpm: $(TARGETS)
	GOARCH=amd64 SEMVER=$(VERSION) nfpm package -p rpm
	GOARCH=arm64 SEMVER=$(VERSION) nfpm package -p rpm

