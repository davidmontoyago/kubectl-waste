# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=kubectl-waste

all: test build

build:
	$(GOBUILD) cmd/kubectl-waste.go && cp ./kubectl-waste /usr/local/bin/

test:
	$(GOTEST) ./pkg/cmd/

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
