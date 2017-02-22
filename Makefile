all: build

FLAGS =
COMMONENVVAR = GOOS=darwin GOARCH=amd64
BUILDENVVAR = CGO_ENABLED=0
TESTENVVAR = 
REGISTRY = gcr.io/google_containers
TAG = $(shell git describe --abbrev=0)

deps:
	go get github.com/tools/godep

build: clean deps
	$(COMMONENVVAR) $(BUILDENVVAR) godep go build -o job_exporter

test-unit: clean deps build
	$(COMMONENVVAR) $(TESTENVVAR) godep go test --race . $(FLAGS)

container: build
	docker build -t ${REGISTRY}/job_exporter:$(TAG) .

push: container
	gcloud docker push ${REGISTRY}/job_exporter:$(TAG)

clean:
	rm -f job_exporter

.PHONY: all deps build test-unit container push clean