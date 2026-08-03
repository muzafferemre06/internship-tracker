.PHONY: help backend frontend test build clean

help:
	@echo "backend  Run the Go API"
	@echo "frontend Run the PWA development server"
	@echo "test     Run backend and frontend tests"
	@echo "build    Build backend and frontend"
	@echo "clean    Remove generated build output"

backend:
	go run ./cmd/api

frontend:
	npm --prefix web run dev

test:
	go test ./...
	npm --prefix web test

build:
	mkdir -p bin
	go build -o bin/api ./cmd/api
	npm --prefix web run build

clean:
	go clean
	npm --prefix web run clean
