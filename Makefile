# wasgeht

build:
	go build -o wasgehtd cmd/wasgehtd/main.go

deps:
	go mod verify
	go mod tidy

clean:
	rm wasgehtd
	rm -rf data/graphs/*

mrproper: clean
	rm -rfi data/*
