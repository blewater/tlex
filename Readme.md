go get github.com/docker/docker/client
go get github.com/docker/docker/pkg/archive

https://github.com/moby/moby/issues/28269
rm -rf ../github.com/docker/docker/vendor/github.com/docker/go-connections/nat
go get github.com/pkg/errors

docker build --rm -t tlhellowebserver:latest .

docker run --rm -it -p 8770:8770 tlhellowebserver:latest