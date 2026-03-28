# Man page generation requires pandoc (https://pandoc.org)
PREFIX ?= $(HOME)/.local

GO_FILES := $(shell find . -name '*.go')
MAN_SRCS := $(wildcard doc/*.1.md)
MAN_PAGES := $(patsubst doc/%.1.md,man/man1/%.1,$(MAN_SRCS))

.PHONY: all build test vet clean install man

all: build

build: orktree

orktree: $(GO_FILES)
	go build -o orktree ./cmd/orktree

test:
	go test ./...

vet:
	go vet ./...

man: $(MAN_PAGES)

man/man1/%.1: doc/%.1.md
	@mkdir -p man/man1
	pandoc -s -t man $< -o $@

install: orktree man
	install -Dm755 orktree $(PREFIX)/bin/orktree
	@for f in man/man1/*.1; do \
		install -Dm644 "$$f" "$(PREFIX)/share/man/man1/$$(basename $$f)"; \
	done

clean:
	rm -f orktree
	rm -rf man/
