# Copyright (c) 2019, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult the
# LICENSE.md file distributed with the sources of this project regarding your
# rights to use or distribute this software.

all: sympi sycontainerize syrun

checkenv-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* is not set"; \
		exit 1; \
	fi

check: checkenv-GOPATH

syrun:
	cd cmd/syrun; go build syrun.go

sympi: cmd/sympi/sympi.go
	cd cmd/sympi; go build sympi.go

sycontainerize: 
	cd cmd/sycontainerize; go build sycontainerize.go

install: check all
	go install ./...
	@cp -f cmd/sympi/sympi_init ${GOPATH}/bin
	@cp -rf etc ${GOPATH}

test: install
	go test ./...

uninstall: check
	@rm -f $(GOPATH)/bin/sympi \
		$(GOPATH)/bin/sycontainerize

clean:
	@rm -f main
	@rm -f cmd/sympi/sympi \
		cmd/syrun/syrun \
		cmd/sympi/main \
		cmd/sycontainerize/sycontainerize \
		cmd/sycontainerize/main

distclean: clean
