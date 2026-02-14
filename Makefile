# wasgeht

test:
	staticcheck ./...
	go test -count=1 --race -v ./...

build:
	go build -o out/wasgehtd cmd/wasgehtd/main.go

deps:
	go mod verify
	go mod tidy

clean:
	rm -f out/*
	rm -rf data/graphs/*

mrproper: clean
	rm -rfi data/*
