build:
	@mkdir -p bin; \
	CGO_ENABLED=0 go build -o bin/docsrv docsrv.go
