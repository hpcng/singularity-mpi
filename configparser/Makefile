# Copyright (c) 2019, Sylabs Inc. All rights reserved.
# This software is licensed under a 3-clause BSD license. Please consult the
# LICENSE.md file distributed with the sources of this project regarding your
# rights to use or distribute this software.

SUBDIRS =
SOURCES = configparser.go

all:
	for src in $(SOURCES); do \
		GOPATH=`pwd`/.. go build $$src; \
	done
	for dir in $(SUBDIRS); do \
		cd $(TOPDIR)/$$dir; GOPATH=`pwd` make; \
	done
