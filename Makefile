VERSION ?= latest
IMAGE_NAME = rodalbuquerque/mutating-webhook

.PHONY: all build push update-deployment deploy

all: build push update-deployment deploy

build:
	docker build -t $(IMAGE_NAME):$(VERSION) .

push:
	docker push $(IMAGE_NAME):$(VERSION)

update-deployment:
	sed -i.bak 's|$(IMAGE_NAME):.*|$(IMAGE_NAME):$(VERSION)|g' deployment.yaml && rm deployment.yaml.bak

deploy:
	kubectl -n mutatingwebhook apply -f deployment.yaml
