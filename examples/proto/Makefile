.PHONY: all
all: lint generate

.PHONY: lint
lint:
	buf lint

.PHONY: generate
generate:
	buf generate

.PHONY: format
format:
	buf format -w

.PHONY: clean
clean:
	rm -rf gen/typescript
