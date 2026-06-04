.PHONY: frontend-install frontend-build build dev-web dev-api

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build

build: frontend-build
	go build -o mintag.exe ./cmd/mintag

dev-web:
	cd frontend && npm run dev

dev-api:
	go run ./cmd/mintag serve
