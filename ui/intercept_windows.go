//go:build windows

package ui

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
	kernel32                    = windows.NewLazySystemDLL("kernel32.dll")
	pGetModuleHandle            = kernel32.NewProc("GetModuleHandleW")
	user32                      = windows.NewLazySystemDLL("user32.dll")
	pRegisterClassEx            = user32.NewProc("RegisterClassExW")
	pCreateWindowEx             = user32.NewProc("CreateWindowExW")
	pDefWindowProc              = user32.NewProc("DefWindowProcW")
	pGetMessage                 = user32.NewProc("GetMessageW")
	pDispatchMessage            = user32.NewProc("DispatchMessageW")
	pShutdownBlockReasonCreate  = user32.NewProc("ShutdownBlockReasonCreate")
	pShutdownBlockReasonDestroy = user32.NewProc("ShutdownBlockReasonDestroy")
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

// shutdownApproved lets WM_QUERYENDSESSION pass through after the user
// has chosen "Stop sync and shut down" and we re-initiate shutdown ourselves.
var shutdownApproved atomic.Bool

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
// It must return quickly — never block here. Dialog work runs in a goroutine.
func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch uint32(message) {
	case wmQueryEndSession:
		if shutdownApproved.Load() {
			return 1 // we chose to shut down — allow
		}
		interceptState.Lock()
		sw := interceptState.sw
		win := interceptState.win
		interceptState.Unlock()
		if sw != nil && sw.IsSyncRunning() {
			// Register a block reason so Windows shows "blocking apps" screen
			// and waits instead of force-killing us.
			reason, _ := windows.UTF16PtrFromString("Backup sync in progress")
			pShutdownBlockReasonCreate.Call(hwnd, uintptr(unsafe.Pointer(reason)))
			// Show the dialog asynchronously — must NOT block the message pump.
			go handleShutdownWhileSyncing(hwnd, win, sw)
			return 0 // FALSE: block shutdown
		}
		return 1 // TRUE: no sync running — allow shutdown

	case wmEndsession:
		if wParam != 0 {
			// Windows is ending the session despite our block (user force-closed).
			interceptState.Lock()
			sw := interceptState.sw
			interceptState.Unlock()
			if sw != nil {
				sw.CancelSync()
			}
		}
		return 0

	default:
		r, _, _ := pDefWindowProc.Call(hwnd, message, wParam, lParam)
		return r
	}
}

// handleShutdownWhileSyncing shows the interception dialog and then either
// removes the shutdown block (user chose to wait) or cancels the sync and
// re-initiates shutdown (user chose to stop and shut down).
func handleShutdownWhileSyncing(hwnd uintptr, win fyne.Window, sw *StatusWindow) {
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

	allowShutdown := <-decided
	dlg.Hide()
	pShutdownBlockReasonDestroy.Call(hwnd)

	if allowShutdown {
		// Set flag before calling shutdown so our own WM_QUERYENDSESSION passes.
		shutdownApproved.Store(true)
		shutdownSystem()
	}
	// If !allowShutdown: block is removed, sync continues, user shuts down later.
}

func runInterceptorLoop() {
	hInst, _, _ := pGetModuleHandle.Call(0) // 0 = current module
	className, _ := syscall.UTF16PtrFromString("WifiSyncInterceptor")
	cb := syscall.NewCallback(wndProc)

	var wc wndclassex
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	wc.lpfnWndProc = cb
	wc.hInstance = hInst
	wc.lpszClassName = className
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	// Create an invisible top-level window. Top-level (non-message-only) windows
	// receive WM_QUERYENDSESSION from the Windows session manager.
	hwnd, _, _ := pCreateWindowEx.Call(
		0,                                  // dwExStyle
		uintptr(unsafe.Pointer(className)), // lpClassName
		uintptr(unsafe.Pointer(className)), // lpWindowName
		0,                                  // dwStyle (invisible, never shown)
		0, 0, 0, 0,                         // x, y, w, h
		0, 0,                               // hWndParent, hMenu
		hInst,                               // hInstance — must match RegisterClassEx
		0,                                  // lpParam
	)
	if hwnd == 0 {
		return // window creation failed; shutdown interception unavailable
	}

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
