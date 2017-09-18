FROM golang:1.9 AS builder

#  Install goupx to strip binaries to the total minimum.
RUN apt-get update \
    && apt-get install -y upx-ucl \
    && go get -v github.com/pwaller/goupx

RUN go get -v github.com/vdobler/ht/...

WORKDIR /go/src/github.com/vdobler/ht/cmd/ht

# Make sure the builtin documentation is a jour.
RUN go run gendoc.go && go run gengui.go

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ht -ldflags "-X main.version=$(git describe) -s"
RUN goupx --strip-binary ht

# Now ht is built, staticaly linked and stripped.
# Ready for use without the layers only needed
# for building:

FROM scratch
COPY --from=builder /go/src/github.com/vdobler/ht/cmd/ht/ht /ht
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo/ /usr/share/zoneinfo/
WORKDIR /app
ENTRYPOINT ["/ht"]
