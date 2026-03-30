# Man page generation requires pandoc (https://pandoc.org)
PREFIX ?= $(HOME)/.local

OUT_DIR := build
GO_FILES := $(shell find . -name '*.go')
MAN_SRCS := $(wildcard doc/*.1.md)
MAN_PAGES := $(patsubst doc/%.1.md,${OUT_DIR}/man/man1/%.1,$(MAN_SRCS))

.PHONY: all build orktree test vet clean install uninstall man

all: build

build: orktree man

orktree: $(GO_FILES)
	mkdir -p build
	go build -o ${OUT_DIR}/orktree ./cmd/orktree

test:
	go test ./...

vet:
	go vet ./...

man: $(MAN_PAGES)

${OUT_DIR}/man/man1/%.1: doc/%.1.md
	@mkdir -p ${OUT_DIR}/man/man1
	pandoc -s -t man $< -o $@

install: build
	install -Dm755 ${OUT_DIR}/orktree $(PREFIX)/bin/orktree
	@for f in ${OUT_DIR}/man/man1/*.1; do \
		install -Dm644 "$$f" "$(PREFIX)/share/man/man1/$$(basename $$f)"; \
	done
	install -Dm644 completions/orktree.bash $(PREFIX)/share/bash-completion/completions/orktree
	install -Dm644 completions/orktree.zsh $(PREFIX)/share/zsh/site-functions/_orktree

uninstall: build
	rm -f $(PREFIX)/bin/orktree
	rm -f $(PREFIX)/share/bash-completion/completions/orktree
	rm -f $(PREFIX)/share/zsh/site-functions/_orktree
	@for f in ${OUT_DIR}/man/man1/*.1; do \
		rm -f "$(PREFIX)/share/man/man1/$$(basename $$f)"; \
	done

clean:
	rm -rf build
