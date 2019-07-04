# Credit: https://medium.com/@adriaandejonge/simplify-the-smallest-possible-docker-image-62c0e0d342ef

# Multi-stage build using go's standard debian img

# 1st build image
FROM golang as builder
# build with standard go compiler:
# static linking pulling a repo service
RUN CGO_ENABLED=0 go get -a -ldflags '-s' github.com/nethatix/echopathws

# 2nd final image to run the built artifact from 1st image

# Docker base image
FROM scratch 
COPY --from=builder /go/bin/echopathws . 
EXPOSE 8770
CMD ["./echopathws"]