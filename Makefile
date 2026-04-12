.PHONY: build install

build:
	go build -o fenec .

install: build
	go install .
