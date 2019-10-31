# Copyright (c) 2019, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult the
# LICENSE.md file distributed with the sources of this project regarding your
# rights to use or distribute this software.

all: sympi syvalidate sycontainerize

syvalidate: 
	cd cmd/syvalidate; go build syvalidate.go

sympi: cmd/sympi/sympi.go
	cd cmd/sympi; go build sympi.go

sycontainerize: 
	cd cmd/sycontainerize; go build sycontainerize.go

install: all
	go install ./...
	@cp -f cmd/sympi/sympi_init ${GOPATH}/bin

uninstall:
	@rm -f $(GOPATH)/bin/sympi \
		$(GOPATH)/bin/syvalidate \
		$(GOPATH)/bin/sycontainerize

clean:
	@rm -f main
	@rm -f cmd/syvalidate/syvalidate \
		cmd/syvalidate/main \
		cmd/sympi/sympi \
		cmd/sympi/main \
		cmd/sycontainerize/sycontainerize \
		cmd/sycontainerize/main

distclean: clean
