.DEFAULT_GOAL := compile

.PHONY: compile
compile:
	mkdir -p ./bin && \
	go build -o ./bin/bufferbloater

.PHONY: test
test: compile
	go test -race -cover ./...
