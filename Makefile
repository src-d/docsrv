# Default shell
SHELL := /bin/sh

# Global CI
LANGUAGES := go
# Global CI. Do not edit
MAKEFILE := Makefile.docs
CI_REPO_URL := https://github.com/src-d/ci.git
SHARED_PATH ?= /etc/shared
CI_PATH ?= $(SHARED_PATH)/ci

$(MAKEFILE):
	@if [ ! -r "./$(MAKEFILE)" ]; then \
		if [ ! -r "$(CI_PATH)/$(MAKEFILE)" ]; then \
			echo "Downloading 'ci'..."; \
			rm -rf "$(CI_PATH)"; \
			mkdir -p "$(CI_PATH)"; \
			git clone "$(CI_REPO_URL)" "$(CI_PATH)"; \
		fi; \
		echo "Installing 'ci'..."; \
		cp $(CI_PATH)/$(MAKEFILE) .; \
	fi; \

-include $(MAKEFILE)

#tools
mkdir := mkdir -p
go_build := CGO_ENABLED=0 go build -o

build:
	@$(mkdir) bin;
	$(go_build) bin/docsrv docsrv.go
