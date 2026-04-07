.PHONY: all
all: build test

.PHONY: build
build:
	go build ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: proto
proto:
	mkdir -p bin
	go build --trimpath -o bin/protoc-gen-typescript-http .
	PATH=$(PWD)/bin:$$PATH $(MAKE) -C examples/proto -f Makefile generate

.PHONY: clean
clean:
	go clean
	rm -rf bin
	$(MAKE) -C examples/proto clean
