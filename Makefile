.PHONY redis-up:
redis-up:
	docker run --name socket-rush-redis --rm -p 6379:6379 redis

.PHONY redis-down:
redis-down:
	docker stop socket-rush-redis
	#docker rm socket-rush-redis
