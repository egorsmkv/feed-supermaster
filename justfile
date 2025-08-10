default: release

build: clean
    go build -race -o feed-master app/main.go

release: clean
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o feed-master -ldflags="-s -w" app/main.go

release_musl: clean
    CC="musl-gcc" LD="musl-ld" CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o feed-master -ldflags="-s -w -extld=musl-gcc" app/main.go

clean:
    rm -f feed-master
