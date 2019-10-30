# Copyright (c) 2019, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult the
# LICENSE.md file distributed with the sources of this project regarding your
# rights to use or distribute this software.

all: sympi syvalidate sycontainerize

syvalidate: cmd/syvalidate/main.go
	cd cmd/syvalidate; go build main.go

sympi: cmd/sympi/main.go
	cd cmd/sympi; go build main.go

sycontainerize: cmd/sycontainerize/main.go
	cd cmd/sycontainerize; go build main.go

install:
	go install ./...

clean:
	@rm -f main

distclean: clean
