all: build

FLAGS =
COMMONENVVAR = GOOS=linux GOARCH=amd64
BUILDENVVAR = CGO_ENABLED=0
TESTENVVAR = 
REGISTRY = 080385600816.dkr.ecr.us-east-1.amazonaws.com
TAG = 0.1.0
LOGIN:=$(shell aws ecr get-login)
PWD:=$(shell pwd)

export GOPATH=${PWD}

deps:
	cd src/mmerrill.io/job-exporter && glide install -v

build: clean deps
	$(COMMONENVVAR) $(BUILDENVVAR) go build -o job-exporter mmerrill.io/job-exporter

test-unit: clean deps build
	$(COMMONENVVAR) $(TESTENVVAR) go test --race . $(FLAGS)

container: build
	docker build -t job-exporter:$(TAG) .

push: container
	exec ${LOGIN}
	docker tag job-exporter:$(TAG) ${REGISTRY}/job-exporter:$(TAG)
	docker push ${REGISTRY}/job-exporter:$(TAG)

clean:
	rm -f job-exporter
	rm -fr src/mmerrill.io/job-exporter/vendor
	rm -f src/mmerrill.io/job-exporter/glide.lock

.PHONY: all deps build test-unit container push clean
