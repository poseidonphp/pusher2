.PHONY redis-up:
redis-up:
	docker run --name socket-rush-redis --rm -p 6379:6379 redis

.PHONY redis-down:
redis-down:
	docker stop socket-rush-redis
	#docker rm socket-rush-redis

.PHONY ui:
ui:
	cd dev/ui && yarn && yarn dev

.PHONY run:
run:
	go run cmd/pusher/main.go

.PHONY run-alt:
run-alt:
	go run cmd/pusher/main.go --port=6002

.PHONY run-alt2:
run-alt2:
	go run cmd/pusher/main.go --port=6003



.PHONY auth:
auth:
	cd dev/auth-server && go run main.go

.PHONY build:
build:
	GOOS=darwin GOARCH=amd64 go build -mod=readonly -ldflags="-s -w" -o pusher-server-mac cmd/pusher/main.go
	GOOS=linux GOARCH=amd64 go build -mod=readonly -ldflags="-s -w" -o pusher-server-x86 cmd/pusher/main.go
	GOOS=linux GOARCH=arm64 go build -mod=readonly -ldflags="-s -w" -o pusher-server-arm64 cmd/pusher/main.go