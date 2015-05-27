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
