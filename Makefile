REGISTRY := linkacloud
IMAGE := oidc-proxy
TAG = $(shell git diff --quiet && git describe --tags --exact-match 2> /dev/null)
VERSION_SUFFIX = $(shell git diff --quiet || echo "-dev")
VERSION ?= $(shell git describe --tags --exact-match 2> /dev/null || echo "`git describe --tags $$(git rev-list --tags --max-count=1) 2> /dev/null || echo v0.0.0`-`git rev-parse --short HEAD`")$(VERSION_SUFFIX)

show-version:
	@echo $(VERSION)

.PHONY:
docker-build:
	@docker image build -t $(REGISTRY)/$(IMAGE):$(VERSION) .

.PHONY:
docker-push:
	@docker image push $(REGISTRY)/$(IMAGE):$(VERSION)


.PHONY:
docker: docker-build docker-push
