all: build

FLAGS =
COMMONENVVAR = GOOS=linux GOARCH=amd64
BUILDENVVAR = CGO_ENABLED=0
TESTENVVAR = 
REGISTRY = 080385600816.dkr.ecr.us-east-1.amazonaws.com
TAG = 0.0.1
LOGIN:=$(shell aws ecr get-login)

deps:
	go get github.com/tools/godep

build: clean deps
	$(COMMONENVVAR) $(BUILDENVVAR) godep go build -o job-exporter

test-unit: clean deps build
	$(COMMONENVVAR) $(TESTENVVAR) godep go test --race . $(FLAGS)

container: build
	#docker build -t job-exporter:$(TAG) .

push: container
	exec ${LOGIN}
	docker tag job-exporter:$(TAG) ${REGISTRY}/job-exporter:$(TAG)
	docker push ${REGISTRY}/job-exporter:$(TAG)

clean:
	rm -f job_exporter

.PHONY: all deps build test-unit container push clean
