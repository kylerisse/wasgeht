# wasgeht

build:
	go build -o wasgeht cmd/wasgeht/main.go

deps:
	go mod verify
	go mod tidy

