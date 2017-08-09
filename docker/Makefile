##
## Makefile to build a tiny standalone Docker image for ht
##

SHELL=/bin/bash

image: ht ca-certificates.crt zoneinfo.tar.gz
	docker build -t vodo/ht:latest .

tag: image
	v=$$(git describe) \
	&& v="$${v#v}" \
	&& docker tag vodo/ht:latest vodo/ht:$$v \
	&& if [[ "$$v" = *.*.*-* ]]; then \
	       echo "Not a taged version."; \
	       exit 1; \
	   else \
	       v="$$(echo "$$v" | sed "s/\.[0-9]\+$$//")" \
	       && docker tag vodo/ht:latest vodo/ht:$$v \
	       && v="$$(echo "$$v" | sed "s/\.[0-9]\+$$//")" \
	       && docker tag vodo/ht:latest vodo/ht:$$v; \
           fi

push: tag
	v=$$(git describe) \
	&& v="$${v#v}" \
	&& docker login \
	&& docker push vodo/ht:latest \
	&& docker push "vodo/ht:$$v" \
	&& v="$$(echo "$$v" | sed "s/\.[0-9]\+$$//")" \
	&& docker push vodo/ht:$$v \
	&& v="$$(echo "$$v" | sed "s/\.[0-9]\+$$//")" \
	&& docker push vodo/ht:$$v \

ht:
	cd ../cmd/ht && \
	go run gendoc.go && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ht -ldflags "-X main.version=$$(git describe) -s"
	cp ../cmd/ht/ht .
	goupx --strip-binary ht

ca-certificates.crt:
	curl -o $@ https://curl.haxx.se/ca/cacert.pem

zoneinfo.tar.gz:
	tar cfz $@ /usr/share/zoneinfo


clean:
	rm -f ht ca-certificates.crt zoneinfo.tar.gz

.PHONY: clean image tag push
