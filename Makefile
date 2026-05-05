.PHONY: build run clean test wlbrowser

build: wlbrowser

run: wlbrowser
	./wlbrowser

wlbrowser:
	@echo "Building wlbrowser (linking GTK/WebKit can take a minute)..."
	go build -v -o wlbrowser ./cmd/wlbrowser
	@echo "Build successful: ./wlbrowser"

clean:
	rm -f wlbrowser

test:
	go test ./...
