.PHONY: build test lint check clean

PLUGIN := opencode-go-quota
DIST := dist

build:
	mkdir -p $(DIST)
	CGO_ENABLED=1 go build -trimpath -buildmode=c-shared -o $(DIST)/$(PLUGIN).so .

test:
	go test ./...

lint:
	go vet ./...

check: test lint build

clean:
	rm -rf $(DIST)
