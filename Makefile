# Makefile for building the Admission Controller webhook demo server + docker image.

.DEFAULT_GOAL := docker-build

IMAGE ?= gpu-resource-toleration-admission-controller:latest

.PHONY: docker-build
docker-build:
	docker build -t $(IMAGE) .

.PHONY: clean
## clean: cleans the binary
clean:
	rm gpu-resource-toleration-admission-controller

.PHONY: help
## help: Prints this help message
help:
	@echo "Usage: \n"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
