sync:
	$(eval DEST := $(shell mktemp -d))
	git clone --depth=1 https://github.com/protocolbuffers/protobuf.git $(DEST)
	cp $(DEST)/src/google/protobuf/*.proto conformance
	cp $(DEST)/conformance/*.proto  conformance
	rm -rf $(DEST)
