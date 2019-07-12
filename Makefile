.PHONY: deps
deps:
	GO111MODULE=off go get golang.org/x/lint/golint

.PHONY: lint
lint: deps
	go vet -all .
	golint -set_exit_status .

.PHONY: test
test:
	go test -v
