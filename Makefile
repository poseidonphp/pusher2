.PHONY redis-up:
redis-up:
	docker run --name socket-rush-redis --rm -p 6379:6379 redis

.PHONY redis-down:
redis-down:
	docker stop socket-rush-redis
	#docker rm socket-rush-redis

.PHONY ui:
ui:
	@if [ ! -f ./dev/ui/.env ]; then \
		echo "Creating .env from env.example"; \
		cp ./dev/ui/env.example ./dev/ui/.env; \
	else \
		echo ".env already exists"; \
	fi
	cd dev/ui && yarn && yarn dev

.PHONY build:
build:
	GOOS=darwin GOARCH=amd64 go build -mod=readonly -ldflags="-s -w" -o pusher-server-mac cmd/pusher/main.go
	GOOS=linux GOARCH=amd64 go build -mod=readonly -ldflags="-s -w" -o pusher-server-x86 cmd/pusher/main.go
	GOOS=linux GOARCH=arm64 go build -mod=readonly -ldflags="-s -w" -o pusher-server-arm64 cmd/pusher/main.go