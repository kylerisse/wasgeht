# wasgeht

build:
	go build -o wasgehtd cmd/wasgehtd/main.go

deps:
	go mod verify
	go mod tidy

clean:
	rm wasgehtd
	rm -rf html/imgs

mrproper: clean
	rm -rfi rrds/*
