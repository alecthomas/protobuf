sync:
	$(eval DEST := $(shell mktemp -d))
	git clone --depth=1 https://github.com/protocolbuffers/protobuf.git $(DEST)
	cp $(DEST)/src/google/protobuf/*.proto conformance
	cp $(DEST)/conformance/*.proto  conformance
	rm -rf $(DEST)

FD_PROTO_FILES = $(wildcard filedescriptors/*.proto)
pb: $(FD_PROTO_FILES:%.proto=%.pb)

%.pb: %.proto
	protoc -o $@ $<

.PHONY: pb sync

clean:
	rm -f conformance/*.proto
	rm -f filedescriptors/*.pb
