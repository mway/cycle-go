.PHONY: test
test:
	go test -v -race -failfast -count 1 ./...

.PHONY: bench
bench:
	go test -v -count 1 -run x -bench . ./...

.PHONY: lint
lint:
	@golangci-lint run --new=false ./...
	@for f in $$(find . -name '*.go'); do \
		gofumpt $$f | diff $$f -; \
	done
	
