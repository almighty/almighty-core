VENDOR_DIR=vendor
ifeq ($(OS),Windows_NT)
include ./.make/Makefile.win
else
include ./.make/Makefile.lnx
endif
SOURCE_DIR ?= .
SOURCES := $(shell find $(SOURCE_DIR) -path $(SOURCE_DIR)/vendor -prune -o -name '*.go' -print)
DESIGN_DIR=design
DESIGNS := $(shell find $(SOURCE_DIR)/$(DESIGN_DIR) -path $(SOURCE_DIR)/vendor -prune -o -name '*.go' -print)
CHECK_DIRS=$(shell go list -f {{.Dir}} ./... | grep -v -E "vendor|app|client|tool/cli")

# Find all required tools:
GIT_BIN := $(shell command -v $(GIT_BIN_NAME) 2> /dev/null)
GLIDE_BIN := $(shell command -v $(GLIDE_BIN_NAME) 2> /dev/null)
GO_BIN := $(shell command -v $(GO_BIN_NAME) 2> /dev/null)
HG_BIN := $(shell command -v $(HG_BIN_NAME) 2> /dev/null)

# Used as target and binary output names... defined in includes
CLIENT_DIR=tool/alm-cli

COMMIT=`git rev-parse HEAD`
BUILD_TIME=`date -u '+%Y-%m-%d_%I:%M:%S%p'`

PACKAGE_NAME:=github.com/almighty/almighty-core

# Pass in build time variables to main
LDFLAGS=-ldflags "-X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}"

# If nothing was specified, run all targets as if in a fresh clone
.PHONY: all
all: prebuild-check deps generate build

.PHONY: build
build: prebuild-check $(BINARY_SERVER) $(BINARY_CLIENT)

$(BINARY_SERVER): prebuild-check $(SOURCES)
	go build -v ${LDFLAGS} -o ${BINARY_SERVER}

$(BINARY_CLIENT): prebuild-check $(SOURCES)
	cd ${CLIENT_DIR} && go build -v -o ../../${BINARY_CLIENT}

# These are binary tools from our vendored packages
$(GOAGEN_BIN): prebuild-check
	cd $(VENDOR_DIR)/github.com/goadesign/goa/goagen && go build -v
$(GO_BINDATA_BIN): prebuild-check
	cd $(VENDOR_DIR)/github.com/jteeuwen/go-bindata/go-bindata && go build -v
$(GO_BINDATA_ASSETFS_BIN): prebuild-check
	cd $(VENDOR_DIR)/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs && go build -v
$(FRESH_BIN): prebuild-check
	cd $(VENDOR_DIR)/github.com/pilu/fresh && go build -v
$(GOIMPORTS_BIN):
	cd $(VENDOR_DIR)/golang.org/x/tools/cmd/goimports && go build -v
$(GOLINT_BIN):
	cd $(VENDOR_DIR)/github.com/golang/lint/golint && go build -v

.PHONY: clean
clean: clean-artifacts clean-generated clean-vendor clean-glide-cache clean-check
	rm -fv check-gopath

.PHONY: clean-artifacts
clean-artifacts:
	rm -fv $(BINARY_SERVER)
	rm -fv $(BINARY_CLIENT)

.PHONY: clean-generated
clean-generated:
	rm -rfv ./app
	rm -rfv ./assets/js
	rm -rfv ./client/
	rm -rfv ./swagger/
	rm -rfv ./tool/cli/
	rm -fv ./bindata_assetfs.go

.PHONY: clean-vendor
clean-vendor:
	rm -rf $(VENDOR_DIR)

.PHONY: clean-glide-cache
clean-glide-cache:
	rm -rf ./.glide

.PHONY: clean-check
clean-check:
	rm -f check_error

# This will download the dependencies
.PHONY: deps
deps: prebuild-check
	$(GLIDE_BIN) install

.PHONY: generate
generate: prebuild-check $(DESIGNS) $(GOAGEN_BIN) $(GO_BINDATA_ASSETFS_BIN) $(GO_BINDATA_BIN)
	$(GOAGEN_BIN) bootstrap -d ${PACKAGE_NAME}/${DESIGN_DIR}
	$(GOAGEN_BIN) js -d ${PACKAGE_NAME}/${DESIGN_DIR} -o assets/ --noexample
	$(GOAGEN_BIN) gen -d ${PACKAGE_NAME}/${DESIGN_DIR} --pkg-path=github.com/goadesign/gorma
	PATH="$(PATH):$(EXTRA_PATH)" $(GO_BINDATA_ASSETFS_BIN) -debug assets/...

.PHONY: dev
dev: prebuild-check $(FRESH_BIN)
	docker-compose up -d
	$(FRESH_BIN)

.PHONY: test-all
test-all: prebuild-check test-unit test-integration

.PHONY: test-unit
test-unit: prebuild-check
	go test $(go list ./... | grep -v vendor) -v -coverprofile coverage-unit.out

.PHONY: test-integration
test-integration: prebuild-check
	go test $(go list ./... | grep -v vendor) -v -dbhost localhost -coverprofile coverage-integration.out -tags=integration

.PHONY: prebuild-check
prebuild-check: $(CHECK_GOPATH_BIN)
# Check that all tools where found
ifndef GIT_BIN
	$(error The "$(GIT_BIN_NAME)" executable could not be found in your PATH)
endif
ifndef GLIDE_BIN
	$(error The "$(GLIDE_BIN_NAME)" executable could not be found in your PATH)
endif
ifndef HG_BIN
	$(error The "$(HG_BIN_NAME)" executable could not be found in your PATH)
endif
	@$(CHECK_GOPATH_BIN) $(PACKAGE_NAME) || (echo "Project lives in wrong location"; exit 1)

$(CHECK_GOPATH_BIN): .make/check-gopath.go
ifndef GO_BIN
	$(error The "$(GO_BIN_NAME)" executable could not be found in your PATH)
endif
	go build .make/check-gopath.go

.PHONY: check
.ONESHELL: check
check: clean-check $(GOIMPORTS_BIN) $(GOLINT_BIN)
	for d in $(CHECK_DIRS) ; do \
		$(GOIMPORTS_BIN) -l $$d/*.go | grep -vEf .golint_exclude >> check_error; \
	done
	for d in $(CHECK_DIRS) ; do \
		$(GOLINT_BIN) $$d | grep -vEf .golint_exclude >> check_error; \
	done
	for d in $(CHECK_DIRS) ; do \
		go tool vet --all $$d/*.go 2>&1 >> check_error; \
	done
	if [ -a check_error ]; then \
		if [ "`cat check_error`" ]; then \
			cat check_error && exit 1; \
		fi
	fi