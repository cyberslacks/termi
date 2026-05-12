BINARY := bin/termi
CMD    := .

.PHONY: build run test test-integration fuzz lint clean install

build:
	go build -o $(BINARY) $(CMD)

run:
	go run $(CMD)

test:
	go test ./... -v -race

test-integration:
	go test ./... -tags integration -v -race

fuzz:
	go test -fuzz=FuzzVTParser ./pkg/vtparser/ -fuzztime=60s

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

install:
	go install $(CMD)

deps:
	go get github.com/charmbracelet/bubbletea@latest
	go get github.com/charmbracelet/lipgloss@latest
	go get github.com/charmbracelet/bubbles@latest
	go get github.com/charmbracelet/x/ansi@latest
	go get golang.org/x/crypto@latest
	go get github.com/zalando/go-keyring@latest
	go get modernc.org/sqlite@latest
	go get github.com/robfig/cron/v3@latest
	go get github.com/anthropics/anthropic-sdk-go@latest
	go get github.com/ollama/ollama@latest
	go get gopkg.in/yaml.v3@latest
	go get github.com/spf13/cobra@latest
	go get github.com/spf13/viper@latest
	go mod tidy
