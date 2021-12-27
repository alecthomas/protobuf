# Run `make help` to display help

all:  build test lint  ## build, test and lint go source

ci: clean check-uptodate all  ## Full clean build and up-to-date checks as run on CI

check-uptodate: sync pb tidy
	test -z "$$(git status --porcelain)"

clean::  ## Remove generated files
	-rm -rf $(O)

.PHONY: all check-uptodate ci clean

# --- Test -------------------------------------------------------------
build:
	go build ./...

test:  ## Test go source
	go test ./...

tidy:
	go mod tidy

lint:  ## Lint go source code
	golangci-lint run

.PHONY: lint test

# --- Conformance ------------------------------------------------------
sync:  ## Clone and copy conformance protos from GitHub
	$(eval DEST := $(shell mktemp -d))
	git clone --depth=1 https://github.com/protocolbuffers/protobuf.git $(DEST)
	cp $(DEST)/src/google/protobuf/*.proto testdata/conformance
	cp $(DEST)/conformance/*.proto  testdata/conformance
	rm -rf $(DEST)

clean::
	rm -rf testdata/conformance/*

.PHONY: sync

# --- Protos -----------------------------------------------------------
COMPILER_PROTO_FILES = $(wildcard compiler/testdata/*.proto)
COMPILER_PB_FILES = $(patsubst compiler/testdata/%.proto,compiler/testdata/pb/%.pb,$(COMPILER_PROTO_FILES))

pb: $(COMPILER_PB_FILES)  ## Generate binary FileDescriptorSet as pb files for compiler/testdata/*.proto

compiler/testdata/pb/%.pb: compiler/testdata/%.proto
	protoc --include_imports -I compiler/testdata -o $@ $<
	# special case: don't include imports
	protoc -I compiler/testdata -o compiler/testdata/pb/06_proto3_import_transitive_no_include.pb compiler/testdata/06_proto3_import_transitive.proto

clean::
	rm -rf compiler/testdata/pb/*.pb

.PHONY: pb

# --- Utilities --------------------------------------------------------
COLOUR_NORMAL = $(shell tput sgr0 2>/dev/null)
COLOUR_WHITE  = $(shell tput setaf 7 2>/dev/null)

help:
	@awk -F ':.*## ' 'NF == 2 && $$1 ~ /^[A-Za-z0-9%_-]+$$/ { printf "$(COLOUR_WHITE)%-25s$(COLOUR_NORMAL)%s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

.PHONY: help

define nl


endef
ifndef ACTIVE_HERMIT
$(eval $(subst \n,$(nl),$(shell bin/hermit env -r | sed 's/^\(.*\)$$/export \1\\n/')))
endif

# Ensure make version is gnu make 3.82 or higher
ifeq ($(filter undefine,$(value .FEATURES)),)
$(error Unsupported Make version. \
	$(nl)Use GNU Make 3.82 or higher (current: $(MAKE_VERSION)). \
	$(nl)Activate 🐚 hermit with `. bin/activate-hermit` and run again \
	$(nl)or use `bin/make`)
endif

