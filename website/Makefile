# SPDX-FileCopyrightText: Copyright The Colima Authors
# SPDX-License-Identifier: Apache-2.0
# See also: package.json
NPM ?= npm

build serve: _output/docsy node_modules/.bin/hugo
	$(NPM) run $@

_output/docsy:
	$(MAKE) -C .. docsy

node_modules/.bin/hugo:
	$(NPM) install

clean:
	$(NPM) run $@
	rm -Rf _output
