##
## Makefile to build a tiny standalone Docker image for ht
##

SHELL=/bin/bash

image:
	cd .. && docker build -f docker/Dockerfile -t vodo/ht:latest .

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

.PHONY: image tag push
