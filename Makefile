all: dev

clean:
	rm -rf _output

dev:
	go get -v ./cmd/colima

release:
	sh release.sh ${VERSION}

gh_release:
	GITHUB=1 sh release.sh ${VERSION} -F CHANGELOG.md
