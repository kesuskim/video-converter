TARGET = video-converter
VERSION = 0.0.2

all: dist

dist: build-mac build-win
	zip -r $(TARGET)_$(VERSION).macOS_universal.zip $(TARGET)_$(VERSION).app
	zip -r $(TARGET)_$(VERSION).windows_x64.zip $(TARGET)_$(VERSION).exe

build-icon:
	go run scripts/generate_icon.go

clean:
	rm -rf $(TARGET)_amd64
	rm -rf $(TARGET)_arm64
	rm -rf $(TARGET)_universal
	rm -rf $(TARGET).exe
	rm -rf $(TARGET)_*.exe
	rm -rf $(TARGET).app
	rm -rf $(TARGET)_*.app
	rm -rf $(TARGET)_*.zip


# TODO
#build-linux:
#	@echo 'Build $(TARGET) for Linux'
#
#	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(TARGET)

build-mac: build-icon
	@echo 'Build $(TARGET) for macOS'

	rm -rf $(TARGET).app
	rm -rf $(TARGET)_*.app

	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.VERSION=$(VERSION)" -p 4 -v -o $(TARGET)_amd64
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w -X main.VERSION=$(VERSION)" -p 4 -v -o $(TARGET)_arm64
	lipo -create -output $(TARGET)_universal $(TARGET)_amd64 $(TARGET)_arm64

	rm $(TARGET)_amd64 $(TARGET)_arm64

	mkdir -p $(TARGET).app/Contents/MacOS
	mkdir -p $(TARGET).app/Contents/Resources

	cp res/mac/app.icns $(TARGET).app/Contents/Resources
	cp res/mac/Info.plist $(TARGET).app/Contents/
	cp $(TARGET)_universal $(TARGET).app/Contents/MacOS/$(TARGET)

	mv $(TARGET).app $(TARGET)_$(VERSION).app


build-win: build-icon
	@echo 'Build $(TARGET) for Windows'

	echo 'id ICON "./res/win/app.ico"' >> $(TARGET).rc
	echo 'GLFW_ICON ICON "./res/win/app.ico"' >> $(TARGET).rc

	x86_64-w64-mingw32-windres $(TARGET).rc -O coff -o $(TARGET).syso
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ HOST=x86_64-w64-mingw32 \
		go build -trimpath -ldflags "-s -w -H=windowsgui -extldflags=-static -X main.VERSION=$(VERSION)" -p 4 -v -o $(TARGET).exe

	rm $(TARGET).syso
	rm $(TARGET).rc

	mv $(TARGET).exe $(TARGET)_$(VERSION).exe

.PHONY: app run clean build-icon

