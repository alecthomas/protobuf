PROTO_FILES = $(wildcard testdata/*.proto)
PB_FILES = $(patsubst testdata/%.proto,testdata/pb/%.pb,$(PROTO_FILES))

pb: $(PB_FILES)

testdata/pb/%.pb: testdata/%.proto
	protoc -I testdata -o $@ $<

check-uptodate:
	test -z "$$(git status --porcelain)"

ci: clean pb check-uptodate

clean:
	rm -f testdata/pb/*.pb

.PHONY: clean pb

