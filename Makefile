SHELL := /bin/bash
TARGETS := webshare

.PHONY: all
all: $(TARGETS)

%: cmd/%/main.go
	go build -ldflags="-s -w" -o $@ $<

.PHONY: clean
clean:
	rm -f $(TARGETS)


