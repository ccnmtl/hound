all: hound

hound: hound.go smtp.go alert.go alertscollection.go config.go
	go build .

fmt:
	go fmt *.go

run: hound
	./hound -config=./config.json
