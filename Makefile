.PHONY: v8-deps-all v8-deps-darwin-arm64 v8-deps-darwin-amd64 v8-deps-linux-arm64 v8-deps-linux-amd64

v8-deps-all:
	./deps/build-all-local.sh all

v8-deps-darwin-arm64:
	./deps/build-all-local.sh darwin-arm64

v8-deps-darwin-amd64:
	./deps/build-all-local.sh darwin-amd64

v8-deps-linux-arm64:
	./deps/build-all-local.sh linux-arm64

v8-deps-linux-amd64:
	./deps/build-all-local.sh linux-amd64
