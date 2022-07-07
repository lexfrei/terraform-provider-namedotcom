.PHONY: $(PLATFORMS) release

NAME := terraform-provider-namedotcom
PLATFORMS ?= darwin/amd64 linux/amd64 windows/amd64 darwin/arm64 linux/arm64 windows/arm64
VERSION ?= $(shell git describe &>/dev/null && echo "_$$(git describe)")

temp = $(subst /, ,$@)
os = $(word 1, $(temp))
arch = $(word 2, $(temp))

BASE := $(NAME)$(VERSION)
RELEASE_DIR := ./release

release: $(PLATFORMS)

$(PLATFORMS):
	GOPROXY="off" GOFLAGS="-mod=vendor" GOOS=$(os) GOARCH=$(arch) go build -o '$(RELEASE_DIR)/$(BASE)-$(os)-$(arch)'

