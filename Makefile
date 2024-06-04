installsqlc:
	curl -L "https://github.com/sqlc-dev/sqlc/releases/download/v1.26.0/sqlc_1.26.0_linux_amd64.tar.gz" | \
		tar -xz -C /usr/local/bin sqlc

dbip:
	curl -L https://download.db-ip.com/free/dbip-city-lite-2024-06.mmdb.gz | gunzip -c > geoip.mmdb

build:
	cd picolytics && sqlc generate
	rm -rf dist && mkdir dist
	cd cmd/picolytics && \
		GIT_COMMIT=$$(git rev-list -1 HEAD --abbrev-commit) \
		GIT_BRANCH=$$(git rev-parse --abbrev-ref HEAD) \
		GOOS=linux GOARCH=amd64 go build \
			-ldflags "-X main.InjectedGitCommit=$$GIT_COMMIT -X main.InjectedGitBranch=$$GIT_BRANCH -X main.InjectedAppVersion=$$APP_VERSION" \
			-o ../../dist/picolytics-$$APP_VERSION-linux-amd64
	mv dist/picolytics-$$APP_VERSION-linux-amd64 dist/picolytics
	tar -czvf dist/picolytics-$$APP_VERSION-linux-amd64.tgz -C dist picolytics
	mv dist/picolytics dist/picolytics-$$APP_VERSION-linux-amd64
	cd cmd/picolytics && \
		GIT_COMMIT=$$(git rev-list -1 HEAD --abbrev-commit) \
		GIT_BRANCH=$$(git rev-parse --abbrev-ref HEAD) \
		GOOS=linux GOARCH=arm64 go build \
			-ldflags "-X main.InjectedGitCommit=$$GIT_COMMIT -X main.InjectedGitBranch=$$GIT_BRANCH -X main.InjectedAppVersion=$$APP_VERSION" \
			-o ../../dist/picolytics-$$APP_VERSION-linux-arm64
	mv dist/picolytics-$$APP_VERSION-linux-arm64 dist/picolytics
	tar -czvf dist/picolytics-$$APP_VERSION-linux-arm64.tgz -C dist picolytics
	mv dist/picolytics dist/picolytics-$$APP_VERSION-linux-arm64
	cd cmd/picolytics && \
		GIT_COMMIT=$$(git rev-list -1 HEAD --abbrev-commit) \
		GIT_BRANCH=$$(git rev-parse --abbrev-ref HEAD) \
		GOOS=darwin GOARCH=arm64 go build \
			-ldflags "-X main.InjectedGitCommit=$$GIT_COMMIT -X main.InjectedGitBranch=$$GIT_BRANCH -X main.InjectedAppVersion=$$APP_VERSION" \
			-o ../../dist/picolytics-$$APP_VERSION-darwin-arm64
	mv dist/picolytics-$$APP_VERSION-darwin-arm64 dist/picolytics
	tar -czvf dist/picolytics-$$APP_VERSION-darwin-arm64.tgz -C dist picolytics
	mv dist/picolytics dist/picolytics-$$APP_VERSION-darwin-arm64

docker:
	docker build -t picolytics \
		--build-arg GIT_COMMIT=$$(git rev-list -1 HEAD --abbrev-commit) \
		--build-arg GIT_BRANCH=$$(git rev-parse --abbrev-ref HEAD) \
		--build-arg APP_VERSION=local \
		--load \
		.

tracker:
	uglifyjs -o cmd/picolytics/static/pico.js cmd/picolytics/static/picolytics.js

load:
	echo requires: https://github.com/tsenart/vegeta/releases/download/v12.11.1/vegeta_12.11.1_linux_amd64.tar.gz
	echo "POST http://localhost:8080/p" | \
		vegeta -cpus 1 attack -duration=10s \
		-rate 50/1s \
		-body etc/vegeta-payload.json \
		| vegeta report -type=json 

	# echo "POST http://localhost:8080/p" 

cover:
	go test -coverprofile=etc/coverage.out ./picolytics/...
	go tool cover -html=etc/coverage.out -o=etc/coverage.html
	open etc/coverage.html

.PHONY: docker tracker cover
