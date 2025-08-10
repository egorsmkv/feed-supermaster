default: build

build: clean
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o feed-master -ldflags="-s -w" app/main.go

build_musl: clean
    CC="musl-gcc" LD="musl-ld" CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o feed-master -ldflags="-s -w -extld=musl-gcc" app/main.go

clean:
    rm -f feed-master
