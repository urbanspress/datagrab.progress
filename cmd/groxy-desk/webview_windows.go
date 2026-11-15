//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"groxy-desk/internal/server"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
)

const (
	csHRedraw     = 0x0002
	csVRedraw     = 0x0001
	wsOverlapped  = 0x00CF0000
	wsVisible     = 0x10000000
	cwUseDefault  = ^uintptr(0x7fffffff)
	swShowDefault = 10
	wmDestroy     = 0x0002
	wmSize        = 0x0005
	idcArrow      = 32512
	coinitApt     = 0x2
)

type wndClassExW struct {
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

type point struct {
	x int32
	y int32
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

func launchWebViewWindow(sess server.SessionCredentials, stateDir string, debug bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_, _, _ = procCoInitializeEx.Call(0, coinitApt)
	defer func() {
		_, _, _ = procCoUninitialize.Call()
	}()

	className, err := windows.UTF16PtrFromString("GroxyDeskWebView2Class")
	if err != nil {
		return err
	}
	title, err := windows.UTF16PtrFromString("Groxy Desk v1.0.0 — Native Vibe-Coding IDE")
	if err != nil {
		return err
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	hCursor, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))

	var chromium *edge.Chromium
	wndProc := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		switch msg {
		case wmSize:
			if chromium != nil {
				chromium.Resize()
			}
			return 0
		case wmDestroy:
			_, _, _ = procPostQuitMessage.Call(0)
			return 0
		default:
			ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
			return ret
		}
	})

	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         csHRedraw | csVRedraw,
		lpfnWndProc:   wndProc,
		hInstance:     hInst,
		hCursor:       hCursor,
		hbrBackground: 6, // COLOR_WINDOW + 1
		lpszClassName: className,
	}
	if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return callErr
	}

	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsOverlapped|wsVisible,
		cwUseDefault,
		cwUseDefault,
		1360,
		860,
		0,
		0,
		hInst,
		0,
	)
	if hwnd == 0 {
		return callErr
	}

	chromium = edge.NewChromium()
	chromium.Debug = debug
	chromium.DataPath = filepath.Join(stateDir, "webview2-profile")

	var embedErr error
	chromium.SetErrorCallback(func(cbErr error) {
		embedErr = cbErr
	})
	chromium.MessageCallback = func(rawMsg string, sender *edge.ICoreWebView2, _ *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		if sender == nil {
			return
		}
		src, srcErr := sender.GetSource()
		if srcErr != nil || (src != sess.URL && !strings.HasPrefix(src, sess.URL+"/")) {
			return
		}
		var incoming struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(rawMsg), &incoming) != nil || incoming.Type != "groxy-desk-ready" {
			return
		}
		payload, _ := json.Marshal(struct {
			Type        string `json:"type"`
			URL         string `json:"url"`
			BearerToken string `json:"bearer_token"`
			CSRFToken   string `json:"csrf_token"`
		}{
			Type:        "groxy-desk-session",
			URL:         sess.URL,
			BearerToken: sess.BearerToken,
			CSRFToken:   sess.CSRFToken,
		})
		_ = sender.PostWebMessageAsString(string(payload))
	}

	if !chromium.Embed(hwnd) {
		if embedErr != nil {
			return embedErr
		}
		return errors.New("failed to embed WebView2 controller")
	}

	_, _, _ = procShowWindow.Call(hwnd, swShowDefault)
	_, _, _ = procUpdateWindow.Call(hwnd)
	_ = chromium.Show()
	chromium.Resize()
	chromium.Navigate(sess.URL + "/")

	var m winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	return nil
}
