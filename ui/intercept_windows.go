//go:build windows

package ui

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sys/windows"
)

const (
	wmQueryEndSession = 0x0011
	wmEndsession      = 0x0016
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	pRegisterClassEx = user32.NewProc("RegisterClassExW")
	pCreateWindowEx  = user32.NewProc("CreateWindowExW")
	pDefWindowProc   = user32.NewProc("DefWindowProcW")
	pGetMessage      = user32.NewProc("GetMessageW")
	pDispatchMessage = user32.NewProc("DispatchMessageW")
)

// wndclassex mirrors WNDCLASSEXW for 64-bit Windows (cbSize = 80).
type wndclassex struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

// winmsg mirrors MSG for 64-bit Windows (GetMessage / DispatchMessage).
type winmsg struct {
	hwnd    uintptr
	message uint32
	// 4 bytes implicit padding (wParam is 8-byte aligned)
	wParam uintptr
	lParam uintptr
	time   uint32
	ptX    int32
	ptY    int32
}

// interceptState holds the pointers accessed by the WndProc callback.
var interceptState struct {
	sync.Mutex
	sw  *StatusWindow
	win fyne.Window
}

// RegisterShutdownInterceptor starts a hidden Windows message window that
// receives WM_QUERYENDSESSION and blocks a shutdown while a sync is running.
func RegisterShutdownInterceptor(win fyne.Window, sw *StatusWindow) {
	interceptState.Lock()
	interceptState.sw = sw
	interceptState.win = win
	interceptState.Unlock()
	go runInterceptorLoop()
}

// wndProc is the window procedure for the hidden interceptor window.
// It runs on the message-pump goroutine and may block for up to 60 s
// while showing the shutdown dialog.
func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch uint32(message) {
	case wmQueryEndSession:
		interceptState.Lock()
		sw := interceptState.sw
		win := interceptState.win
		interceptState.Unlock()
		if sw != nil && sw.IsSyncRunning() {
			return handleShutdownWhileSyncing(win, sw)
		}
		return 1 // TRUE: no sync running — allow shutdown immediately

	case wmEndsession:
		return 0

	default:
		r, _, _ := pDefWindowProc.Call(hwnd, message, wParam, lParam)
		return r
	}
}

// handleShutdownWhileSyncing shows a blocking dialog and returns 1 (allow) or
// 0 (block) for WM_QUERYENDSESSION. Defaults to allow+cancel after 60 s.
func handleShutdownWhileSyncing(win fyne.Window, sw *StatusWindow) uintptr {
	decided := make(chan bool, 1)
	ctx, cancel := context.WithCancel(context.Background())

	var once sync.Once
	decide := func(allowShutdown bool) {
		once.Do(func() {
			cancel()
			if allowShutdown {
				sw.CancelSync()
			}
			decided <- allowShutdown
		})
	}

	countLabel := widget.NewLabel(fmt.Sprintf("Defaulting to 'Stop sync and shut down' in %d seconds...", 60))
	content := container.NewVBox(
		widget.NewLabel("A backup sync is in progress. What would you like to do?"),
		countLabel,
		container.NewHBox(
			widget.NewButton("Wait for sync", func() { decide(false) }),
			widget.NewButton("Stop sync and shut down", func() { decide(true) }),
		),
	)
	dlg := dialog.NewCustomWithoutButtons("System Shutdown Detected", content, win)
	dlg.Show()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for remaining := 60; remaining > 0; remaining-- {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				countLabel.SetText(fmt.Sprintf("Defaulting to 'Stop sync and shut down' in %d seconds...", remaining-1))
			}
		}
		decide(true) // default: cancel sync, allow shutdown
	}()

	result := <-decided
	dlg.Hide()
	if result {
		return 1 // TRUE: allow shutdown
	}
	return 0 // FALSE: block shutdown
}

func runInterceptorLoop() {
	className, _ := syscall.UTF16PtrFromString("WifiSyncInterceptor")
	cb := syscall.NewCallback(wndProc)

	var wc wndclassex
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	wc.lpfnWndProc = cb
	wc.lpszClassName = className
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	// Create an invisible top-level window. Top-level (non-message-only) windows
	// receive WM_QUERYENDSESSION from the Windows session manager.
	pCreateWindowEx.Call(
		0,                                      // dwExStyle
		uintptr(unsafe.Pointer(className)),     // lpClassName
		uintptr(unsafe.Pointer(className)),     // lpWindowName
		0,                                      // dwStyle (WS_OVERLAPPED, never shown)
		0, 0, 0, 0,                             // x, y, w, h
		0, 0, 0, 0,                             // hWndParent, hMenu, hInstance, lpParam
	)

	var m winmsg
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		// GetMessage returns 0 for WM_QUIT and 0xFFFFFFFF (−1 zero-extended) on error.
		if r == 0 || r == uintptr(0xFFFFFFFF) {
			break
		}
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}
