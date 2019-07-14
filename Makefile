.PHONY: build

build:
	@CGO_ENABLED=0 GOOS=darwin go build -o terraform-provider-namedotcom

