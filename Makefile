REGISTRY := linkacloud
IMAGE := oidc-proxy
VERSION  := latest

.PHONY:
docker-build:
	@docker image build -t $(REGISTRY)/$(IMAGE):$(VERSION) .

.PHONY:
docker-push:
	@docker image push $(REGISTRY)/$(IMAGE):$(VERSION)


.PHONY:
docker: docker-build docker-push
