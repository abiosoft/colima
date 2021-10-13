all: dev

clean:
	rm -rf _output

dev:
	go get -v ./cmd/colima

release:
	sh scripts/release.sh ${VERSION}

gh_release:
	GITHUB=1 sh scripts/release.sh ${VERSION} -F CHANGELOG.md

install: clean release
	cp _output/colima-amd64 /usr/local/bin/colima
	chmod +x /usr/local/bin/colima
