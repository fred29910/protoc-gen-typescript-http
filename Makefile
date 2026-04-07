.PHONY: all
all: build test

.PHONY: build
build:
	mage build

.PHONY: test
test:
	mage test

.PHONY: integration
integration:
	mage integration

.PHONY: clean
clean:
	mage clean
