BIN := bin/spectreps
PKG := ./cmd/spectreps

.PHONY: help build test lint fmt tidy clean

help:
	@printf '%s\n' \
		'build  compile $(BIN)' \
		'test   go test ./...' \
		'lint   gofmt check and go vet' \
		'fmt    gofmt -w .' \
		'tidy   go mod tidy' \
		'clean  remove bin/'

build:
	go build -trimpath -o $(BIN) $(PKG)

test:
	go test ./...

lint:
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		printf '%s\n' "gofmt needed:" $$files; \
		exit 1; \
	fi
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf bin
