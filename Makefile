ROOT_DIR:=$(dir $(realpath $(lastword $(MAKEFILE_LIST))))

all: hound

hound: hound.go smtp.go alert.go alertscollection.go config.go
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' .

fmt:
	go fmt *.go

test:
	go test .

coverage: coverage.html

coverage.out: *.go
	go test . -coverprofile=coverage.out

coverage.html: coverage.out
	go tool cover -html=coverage.out -o coverage.html

build:
	docker build -t ccnmtl/hound .

push: build
	docker push ccnmtl/hound

.PHONY: all fmt run test coverage build push
