BIN := board

.PHONY: run build test vet tidy clean install

run: ## Run the board (interactive; needs a real terminal)
	go run ./cmd/board

build: ## Build the binary to ./board
	go build -o $(BIN) ./cmd/board

test: ## Run all tests
	go test ./...

vet: ## Static checks
	go vet ./...

tidy: ## Sync go.mod/go.sum
	go mod tidy

install: ## Install `board` onto PATH (GOBIN / GOPATH/bin)
	go install ./cmd/board

clean:
	rm -f $(BIN)
