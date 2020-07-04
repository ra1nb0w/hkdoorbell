GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run

VERSION=$(shell git describe --exact-match --tags 2>/dev/null)
BUILD_DIR=build
PACKAGE_RPI=hkdoorbell-$(VERSION)_linux_armhf

export GO111MODULE=on

.DEFAULT_GOAL := run

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm hkdoorbell

run:
	$(GORUN) cmd/hkdoorbell/main.go

package-rpi: build-rpi
	tar -cvzf $(PACKAGE_RPI).tar.gz -C $(BUILD_DIR) $(PACKAGE_RPI)

build-rpi:
	GOOS=linux GOARCH=arm GOARM=6 $(GOBUILD) -o $(BUILD_DIR)/$(PACKAGE_RPI)/usr/bin/hkdoorbell -i cmd/hkdoorbell/main.go

bin:
	$(GOBUILD) -o hkdoorbell -i cmd/hkdoorbell/main.go

