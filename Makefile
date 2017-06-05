#tools
mkdir := mkdir -p
go_build := CGO_ENABLED=0 go build -o

DOCSRV_PATH ?= $(shell pwd)

build:
	@$(mkdir) bin;
	$(go_build) bin/docsrv $(DOCSRV_PATH)/docsrv.go
