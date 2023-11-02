build: clean
	mkdir -p bin
	go build -mod=vendor -o ./bin/ggcode main.go

darwin: clean
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -mod=vendor -o ./bin/ggcode main.go

fmt:
	go fmt ./...
	goimports -local github.com/ldnvnbl/gg -w `go list -f {{.Dir}} ./...`

clean:
	rm -rf bin api proto service