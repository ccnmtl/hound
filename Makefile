ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all: hound

hound: hound.go smtp.go alert.go alertscollection.go config.go
	go build .

fmt:
	go fmt *.go

run: hound
	./run.sh

test:
	go test .

coverage:
	go test . -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

build:
	docker run --rm -v $(ROOT_DIR):/src -v /var/run/docker.sock:/var/run/docker.sock centurylink/golang-builder ccnmtl/hound

push: build
	docker push ccnmtl/hound
