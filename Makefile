# PokéClaude — atalhos de build/instalação.
# Uso: `make`, `make build`, `make install`, `make arena`, etc.

BINARY      ?= pokeclaude
PKG         := ./cmd/pokeclaude
PREFIX      ?= /usr/local
BINDIR      ?= $(PREFIX)/bin
GO          ?= go
LDFLAGS     := -s -w

.DEFAULT_GOAL := help

## help: lista os targets disponíveis
.PHONY: help
help:
	@echo "PokéClaude — targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: compila o binário (modo texto/TUI, sem raylib)
.PHONY: build
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

## build-gfx: compila com o render gráfico da Arena (requer libs OpenGL/X11/Wayland)
.PHONY: build-gfx
build-gfx:
	$(GO) build -tags raylib -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

## install: instala o binário em $(BINDIR) e injeta os hooks do Claude Code
.PHONY: install
install: build
	install -d $(BINDIR)
	install -m 0755 $(BINARY) $(BINDIR)/$(BINARY)
	$(BINDIR)/$(BINARY) install

## uninstall: remove os hooks e o binário instalado
.PHONY: uninstall
uninstall:
	-$(BINDIR)/$(BINARY) uninstall
	-rm -f $(BINDIR)/$(BINARY)

## doctor: valida socket, settings e terminal
.PHONY: doctor
doctor: build
	./$(BINARY) doctor

## tui: sobe o daemon + interface (foreground)
.PHONY: tui
tui: build
	./$(BINARY) tui

## demo: roda a interface com eventos sintéticos (sem Claude Code)
.PHONY: demo
demo: build
	./$(BINARY) demo

## arena: roda uma rinha headless (modo game, texto)
.PHONY: arena
arena: build
	./$(BINARY) arena

## test: roda os testes
.PHONY: test
test:
	$(GO) test ./...

## race: roda os testes com o detector de corrida
.PHONY: race
race:
	$(GO) test -race ./...

## vet: go vet
.PHONY: vet
vet:
	$(GO) vet ./...

## fmt: formata o código
.PHONY: fmt
fmt:
	gofmt -w .

## check: fmt + vet + test (validação completa)
.PHONY: check
check: fmt vet test

## clean: remove o binário compilado
.PHONY: clean
clean:
	rm -f $(BINARY)
