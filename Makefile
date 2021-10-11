all: dev

dev:
	go get -v ./cmd/colima

release:
	sh release.sh