# wasgeht

build:
	go build -o wasgehtd cmd/wasgehtd/main.go
	go build -o wasgeht-grapher cmd/wasgeht-grapher/main.go

deps:
	go mod verify
	go mod tidy

