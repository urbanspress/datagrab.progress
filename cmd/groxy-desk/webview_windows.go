//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"groxy-desk/internal/server"
	"groxy-desk/internal/workspace"

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
	procSetWindowTextW   = user32.NewProc("SetWindowTextW")
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

func launchWebViewWindow(sess server.SessionCredentials, stateDir, workspaceDir string, debug bool) error {
	return launchWebViewWindowWithController(sess, stateDir, workspaceDir, debug, edge.NewChromium)
}

func launchWebViewWindowWithController(sess server.SessionCredentials, stateDir, workspaceDir string, debug bool, newController func() *edge.Chromium) error {
	files, err := workspace.NewManager(workspaceDir, "", "", nil)
	if err != nil {
		return err
	}
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
	title, err := windows.UTF16PtrFromString("Groxy Desk v1.4.0 — Native Vibe-Coding IDE")
	if err != nil {
		return err
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	hCursor, _, _ := procLoadCursorW.Call(0, uintptr(idcArrow))

	var chromium, research *edge.Chromium
	var workbenchPane, researchPane uintptr
	width := 45
	activeOrigin := ""
	var browserSession uint64
	var brandPreview struct {
		page, organization string
	}
	selectionHandler := &browserSelectionHandler{vtable: &browserSelectionVtable}
	defer runtime.KeepAlive(selectionHandler)
	resize := func(hwnd uintptr) {
		percent := 0
		if activeOrigin != "" {
			percent = width
		}
		resizePanes(hwnd, workbenchPane, researchPane, percent)
		if chromium != nil {
			chromium.Resize()
		}
		if research != nil {
			research.Resize()
		}
	}
	wndProc := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		switch msg {
		case wmSize:
			if workbenchPane != 0 {
				resize(hwnd)
			}
		case wmDestroy:
			_, _, _ = procPostQuitMessage.Call(0)
			return 0
		}
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
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
		if !errors.Is(callErr, windows.ERROR_CLASS_ALREADY_EXISTS) {
			return callErr
		}
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

	workbenchPane, err = childPane(hwnd, hInst)
	if err != nil {
		return err
	}
	researchPane, err = childPane(hwnd, hInst)
	if err != nil {
		return err
	}
	resize(hwnd)

	openPane := func(parent uintptr, profile string, debug bool, args ...string) (*edge.Chromium, error) {
		pane := newController()
		pane.Debug = debug
		pane.DataPath = filepath.Join(stateDir, profile)
		pane.AdditionalBrowserArgs = append(pane.AdditionalBrowserArgs, args...)
		var embedErr error
		pane.SetErrorCallback(func(err error) { embedErr = err })
		if !pane.Embed(parent) || embedErr != nil {
			if embedErr != nil {
				return nil, embedErr
			}
			return nil, errors.New("failed to embed WebView2 controller")
		}
		return pane, nil
	}
	chromium, err = openPane(workbenchPane, "webview2-profile", debug)
	if err != nil {
		return err
	}
	releaseWorkbench, err := isolateBrowser(chromium,
		func(raw string) bool { return trustedWorkbenchSource(raw, sess.URL) },
		func(raw string) bool { return trustedWorkbenchResource(raw, sess.URL) }, false, nil)
	if err != nil {
		return err
	}
	defer releaseWorkbench()
	origins := parseBrowserOrigins(os.Getenv("GROXY_DESK_BROWSER_ORIGINS"), os.Getenv("GROXY_DESK_CMS_ORIGIN"))
	var proxy *researchProxy
	if len(origins) > 0 {
		proxy, err = startResearchProxy()
		if err != nil {
			return err
		}
		defer proxy.close()
		// Keep even loopback URLs behind the proxy and disable alternate
		// transports or TLS connection coalescing across origins.
		research, err = openPane(researchPane, "webview2-research-profile", false,
			"--proxy-server=http://"+proxy.addr, "--proxy-bypass-list=<-loopback>",
			"--disable-quic", "--disable-http2", "--force-webrtc-ip-handling-policy=disable_non_proxied_udp")
		if err != nil {
			return err
		}
		research.SetGlobalPermission(edge.CoreWebView2PermissionStateDeny)
		allowsResearch := func(raw string) bool {
			return activeOrigin != "" && origins.match(raw) == activeOrigin
		}
		releaseResearch, err := isolateBrowser(research, allowsResearch, allowsResearch, true, func() {
			browserSession++
			brandPreview.page, brandPreview.organization = "", ""
		})
		if err != nil {
			return err
		}
		defer releaseResearch()
		_ = research.Hide()
	}
	notify := func(sender *edge.ICoreWebView2, payload any) {
		raw, err := json.Marshal(payload)
		if err == nil {
			_ = sender.PostWebMessageAsString(string(raw))
		}
	}
	setBrowserState := func(sender *edge.ICoreWebView2, origin, message string) {
		browserSession++
		brandPreview.page, brandPreview.organization = "", ""
		activeOrigin = origin
		if proxy != nil {
			proxy.setOrigin(origin)
		}
		windowTitle := title
		if origin != "" {
			windowTitle, _ = windows.UTF16PtrFromString("Groxy Desk - " + origin)
		}
		_, _, _ = procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(windowTitle)))
		runtime.KeepAlive(windowTitle)
		resize(hwnd)
		notify(sender, map[string]string{"type": "groxy-browser-status", "origin": origin, "message": message})
	}

	chromium.MessageCallback = func(raw string, sender *edge.ICoreWebView2, args *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		// WebMessageReceived on ICoreWebView2 is the top-level handler. Match both
		// the message's document and the controller's current source.
		if sender == nil || args == nil {
			return
		}
		source, err := args.GetSource()
		current, currentErr := sender.GetSource()
		if err != nil || currentErr != nil || !trustedWorkbenchSource(source, sess.URL) || !trustedWorkbenchSource(current, sess.URL) {
			return
		}
		cmd, ok := decodePaneCommand(raw)
		if !ok {
			return
		}
		switch cmd.Type {
		case "groxy-desk-ready":
			notify(sender, map[string]string{"type": "groxy-desk-session", "url": sess.URL, "bearer_token": sess.BearerToken, "csrf_token": sess.CSRFToken})
		case "groxy-browser-open":
			origin := origins.match(cmd.URL)
			if origin == "" || research == nil {
				notify(sender, map[string]string{"type": "groxy-browser-status", "origin": activeOrigin, "message": "GRAB-E402: This browser origin is outside the configured HTTPS allowlist."})
				return
			}
			setBrowserState(sender, origin, "")
			_ = research.Show()
			research.Navigate(cmd.URL)
		case "groxy-browser-close":
			if research != nil {
				_ = research.Hide()
			}
			setBrowserState(sender, "", "")
		case "groxy-browser-resize":
			width = cmd.Width
			resize(hwnd)
		case "groxy-browser-capture", "groxy-browser-brand-preview", "groxy-browser-brand-prefill":
			if research == nil || activeOrigin == "" || selectionHandler.complete != nil {
				return
			}
			view, err := research.GetController().GetCoreWebView2()
			if err != nil {
				return
			}
			defer view.Release()
			page, err := view.GetSource()
			origin := activeOrigin
			if err != nil || origins.match(page) != origin {
				return
			}
			operation := readBrowserSelection
			switch cmd.Type {
			case "groxy-browser-brand-preview":
				brandPreview.page, brandPreview.organization = "", ""
				file, readErr := files.ReadFile(brandDNAPath)
				organization, parseErr := brandOrganization(file.Content)
				if readErr != nil || parseErr != nil {
					notify(sender, map[string]string{"type": "groxy-browser-brand", "message": "Set a business name in the Groxy AI Brand DNA wizard before using prefill."})
					return
				}
				brandPreview.page, brandPreview.organization = page, organization
				notify(sender, map[string]string{"type": "groxy-browser-brand", "origin": origin, "organization": organization})
				return
			case "groxy-browser-brand-prefill":
				preview := brandPreview
				brandPreview.page, brandPreview.organization = "", ""
				if preview.page != page || preview.organization == "" {
					notify(sender, map[string]string{"type": "groxy-browser-brand", "message": "The page changed. Review the business name again before filling."})
					return
				}
				operation = prefillBrandOrganization(page, preview.organization)
			}
			session := browserSession
			selectionHandler.complete = func(text string) {
				if session != browserSession {
					return
				}
				currentView, err := research.GetController().GetCoreWebView2()
				if err != nil {
					return
				}
				defer currentView.Release()
				if current, err := currentView.GetSource(); err != nil || current != page {
					return
				}
				workbenchView, err := chromium.GetController().GetCoreWebView2()
				if err != nil {
					return
				}
				defer workbenchView.Release()
				if current, err := workbenchView.GetSource(); err != nil || !trustedWorkbenchSource(current, sess.URL) {
					return
				}
				if cmd.Type == "groxy-browser-brand-prefill" {
					message := "No single empty organization field was available."
					if text == "filled" {
						message = "Business name filled. Review the form in the browser before submitting."
					}
					notify(workbenchView, map[string]string{"type": "groxy-browser-brand", "message": message})
					return
				}
				notify(workbenchView, map[string]string{"type": "groxy-browser-selection", "origin": origin, "text": text})
			}
			if err := runFixedBrowserOperation(view, operation, selectionHandler); err != nil {
				selectionHandler.complete = nil
				notify(sender, map[string]string{"type": "groxy-browser-status", "origin": origin, "message": "Could not complete the requested browser action."})
			}
		}
	}

	_, _, _ = procShowWindow.Call(hwnd, swShowDefault)
	_, _, _ = procUpdateWindow.Call(hwnd)
	_ = chromium.Show()
	resize(hwnd)
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
