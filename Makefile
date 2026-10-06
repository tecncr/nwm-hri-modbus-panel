.PHONY: ui test build clean

ui:
	cd cmd/nwm-panel/web && npm ci && npm run build

test:
	go test ./...
	cd cmd/nwm-panel/web && npm ci && npm test

build: ui
	mkdir -p build
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o build/nwm-panel ./cmd/nwm-panel

clean:
	rm -rf build
