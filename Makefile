darwin: clean
	packr2
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -mod=vendor -o ./bin/gg main.go
	#./bin/gg --objectName user --objectIdName uid --objectIdType uint64 --module 'github.com/ldnvnbl/gg'
	./bin/gg --objectName chatThread

fmt:
	go fmt ./...
	goimports -local github.com/ldnvnbl/gg -w `go list -f {{.Dir}} ./...`

clean:
	rm -rf bin
	packr2 clean