.PHONY: all build build-linux build-win test vet clean

APP_NAME := llm-router
CMD_DIR := ./cmd/router

all: vet test build

build:
	go build -o $(APP_NAME) $(CMD_DIR)

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(APP_NAME) $(CMD_DIR)

build-win:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(APP_NAME).exe $(CMD_DIR)

test:
	go test ./... -race -count=1

vet:
	go vet ./...

clean:
	rm -f $(APP_NAME) $(APP_NAME).exe
	rm -rf /tmp/llm-router-*