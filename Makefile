# wasgeht

build:
	go build -o wasgehtd cmd/wasgehtd/main.go

deps:
	go mod verify
	go mod tidy

