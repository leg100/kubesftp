LD_FLAGS = '-s -w -X github.com/leg100/kubesftp.Version=edge'

.PHONY: test
test:
	go test ./... -v

.PHONY: build
build:
	go build -ldflags $(LD_FLAGS) -o _build/linux/amd64/ ./cmd/...

# Run staticcheck metalinter recursively against code
.PHONY: lint
lint:
	go tool staticcheck ./...

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

.PHONY: image
image:
	docker build -t kubesftp:edge .

.PHONY: deploy
deploy:
	helm upgrade -i sftp ./charts/kubesftp --wait

.PHONY: load
load: image
	kind load docker-image kubesftp:edge

# Install pre-commit
.PHONY: install-pre-commit
install-pre-commit:
	pip install pre-commit==4.5.1
	pre-commit install

check-no-diff: helm-docs
	git diff --exit-code

.PHONY: deploy-otfd
deploy-otfd:
	helm upgrade -i --create-namespace -n otfd-test -f ./charts/otfd/test-values.yaml otfd ./charts/otfd --wait

.PHONY: test-otfd
test-otfd: deploy-otfd
	helm test -n otfd-test otfd

.PHONY: bump-chart-version
bump-chart-version:
	yq -i '.version |= (split(".") | .[-1] |= ((. tag = "!!int") + 1) | join("."))' ./charts/${CHART}/Chart.yaml

.PHONY: helm-docs
helm-docs:
	go tool helm-docs -c ./charts -u

.PHONY: helm-lint
helm-lint:
	./hack/helm-lint.sh

.PHONY: livereload
livereload:
	wgo ./hacks/reload.sh
