GO_FILES=`go list ./... | grep -v -E "mock|store|test|fake|cmd"`

root=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
store_dsn='root:@tcp(localhost:3306)/test'
endpoint='http://localhost:4572'

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: test
test:
	APP_ENVIRONMENT=test \
	STORE_DSN=$(store_dsn) \
	LT_STORE_DSN=$(store_dsn) \
	PERF_STORE_DSN=$(store_dsn) \
	YONGLE_STORE_DSN=$(store_dsn) \
	QILIN_STORE_DSN=$(store_dsn) \
	LOCAL_S3_ENDPOINT=$(endpoint) CGO_ENABLED=0 go test -v -count=1 -timeout=900s $(GO_FILES)

.PHONY: ci-test
ci-test:
	@go test $(GO_FILES) -coverprofile .cover.txt
	@go tool cover -func .cover.txt
	@rm .cover.txt

.PHONY: ci-migrate
ci-migrate:
	APP_ENVIRONMENT=test go run main.go task ci_migrate

.PHONY: build
build:
	rm -rf bin
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/fermi -ldflags "-X github.com/ops/images-sync/pod.appRelease=${release}" main.go

.PHONY: boot
boot:
	docker-compose up --no-recreate -d

.PHONY: clean
clean:
	docker-compose down --remove-orphans
	docker rm -f $(docker ps -a | grep Exit | awk '{ print $1 }') || true
