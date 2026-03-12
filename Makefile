export PKG_CONFIG_PATH := $(CURDIR)/pkgconfig:$(PKG_CONFIG_PATH)

build:
	go build -o second-nature .

run: build
	@./second-nature

clean:
	rm -f second-nature
