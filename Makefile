all: dev

clean:
	rm -rf _output

dev:
	go get -v ./cmd/colima

release:
	export GITHUB=${GITHUB}
	sh release.sh ${VERSION}
