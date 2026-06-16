BINARY      = wifisync
WIN_BINARY  = wifisync.exe
WIN_LDFLAGS = -ldflags="-H windowsgui -s -w"

.PHONY: windows build clean

windows:
	~/go/bin/fyne-cross windows -arch amd64

build:
	GOTOOLCHAIN=local go build -o $(BINARY) .

clean:
	rm -f $(BINARY) $(WIN_BINARY)
