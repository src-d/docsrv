# Default shell
SHELL := /bin/sh

# Global CI
LANGUAGES := go
# Global CI. Do not edit
MAKEFILE_DOCS := Makefile.docs
CI_REPO_URL := https://github.com/src-d/ci.git
SHARED_PATH ?= $(shell pwd)/.shared
CI_PATH ?= $(SHARED_PATH)/ci

MAKEFILE_DEV := Makefile.dev
MAKEFILE_DEV_PATH := docs/site-generator/$(MAKEFILE_DEV)

$(MAKEFILE_DOCS):
	@if [ ! -r "./$(MAKEFILE_DOCS)" ]; then \
		if [ ! -r "$(CI_PATH)/$(MAKEFILE_DOCS)" ]; then \
			echo "Downloading 'ci'..."; \
			rm -rf "$(CI_PATH)"; \
			mkdir -p "$(CI_PATH)"; \
			git clone "$(CI_REPO_URL)" "$(CI_PATH)"; \
		fi; \
	fi;
	@echo "Installing CI...";
	@cp $(CI_PATH)/$(MAKEFILE_DOCS) .;
	@echo "Installing 'develop' rule...";
	@cp $(CI_PATH)/$(MAKEFILE_DEV_PATH) .;

-include $(MAKEFILE) $(MAKEFILE_DEV)

#tools
mkdir := mkdir -p
go_build := CGO_ENABLED=0 go build -o

build:
	@$(mkdir) bin;
	$(go_build) bin/docsrv docsrv.go
