//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

var (
	procGetClientRect = user32.NewProc("GetClientRect")
	procMoveWindow    = user32.NewProc("MoveWindow")
	procCoTaskMemFree = ole32.NewProc("CoTaskMemFree")
)

type clientRect struct{ left, top, right, bottom int32 }

// ICoreWebView2_22 exposes source-aware filters. The older filter API does
// not cover all frame and worker requests.
var webViewSourceFilterIID = windows.GUID{Data1: 0xdb75dfc7, Data2: 0xa857, Data3: 0x4632, Data4: [8]byte{0xa3, 0x98, 0x69, 0x69, 0xdd, 0xe2, 0x6c, 0x0a}}

func filterAllBrowserRequests(view *edge.ICoreWebView2) (func(), error) {
	var filter unsafe.Pointer
	hr, _, _ := syscall.SyscallN((*(**[3]uintptr)(unsafe.Pointer(view)))[0], uintptr(unsafe.Pointer(view)), uintptr(unsafe.Pointer(&webViewSourceFilterIID)), uintptr(unsafe.Pointer(&filter)))
	if int32(hr) < 0 || filter == nil {
		return nil, errors.New("WebView2 runtime cannot filter all request sources")
	}
	vtable := *(**[5]uintptr)(filter)
	pattern := [...]uint16{'*', 0}
	const allSourceKinds = 0xffffffff
	apply := func(slot int, context uintptr) uintptr {
		hr, _, _ := syscall.SyscallN(vtable[slot], uintptr(filter), uintptr(unsafe.Pointer(&pattern[0])), context, allSourceKinds)
		runtime.KeepAlive(&pattern)
		return hr
	}
	release := func() { _, _, _ = syscall.SyscallN(vtable[2], uintptr(filter)) }
	if hr := apply(3, edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL); int32(hr) < 0 {
		release()
		return nil, fmt.Errorf("register WebView2 request filter: %w", windows.Errno(hr))
	}
	return func() {
		_ = apply(4, edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL)
		release()
	}, nil
}

func childPane(parent, instance uintptr) (uintptr, error) {
	class, _ := windows.UTF16PtrFromString("STATIC")
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0x40000000|wsVisible|0x04000000, 0, 0, 0, 0, parent, 0, instance, 0)
	runtime.KeepAlive(class)
	if hwnd == 0 {
		return 0, err
	}
	return hwnd, nil
}

func resizePanes(parent, workbench, research uintptr, percent int) {
	var rect clientRect
	if ok, _, _ := procGetClientRect.Call(parent, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return
	}
	width := uintptr(rect.right)
	left := width * uintptr(100-percent) / 100
	_, _, _ = procMoveWindow.Call(workbench, 0, 0, left, uintptr(rect.bottom), 1)
	_, _, _ = procMoveWindow.Call(research, left, 0, width-left, uintptr(rect.bottom), 1)
}

// These handlers use the base ICoreWebView2 vtable slots from WebView2.h.
// The Go wrapper provides settings and resource filtering but omits these events.
const (
	addNavigationStarting      = 7
	addFrameNavigationStarting = 17
	addNewWindowRequested      = 44
	addWebResourceRequested    = 55
	removeWebResourceFilter    = 58
	navigationURI              = 3
	navigationCancel           = 8
	newWindowHandled           = 6
	executeScript              = 29
)

// Only the user's top-level text selection may leave the research pane. The
// script is fixed, takes no data, reads no form fields and never posts messages.
const readBrowserSelection = `(() => window.getSelection()?.toString().slice(0, 4000) ?? "")()`

type browserSelectionHandler struct {
	vtable   *[4]uintptr
	complete func(string)
}

var browserSelectionVtable = [4]uintptr{
	syscall.NewCallback(func(self *browserSelectionHandler, iid *windows.GUID, out *unsafe.Pointer) uintptr {
		if out == nil {
			return 0x80004003
		}
		*out = nil
		if iid == nil || (*iid != (windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}) &&
			*iid != (windows.GUID{Data1: 0x49511172, Data2: 0xcc67, Data3: 0x4bca, Data4: [8]byte{0x99, 0x23, 0x13, 0x71, 0x12, 0xf4, 0xc4, 0xcc}})) {
			return 0x80004002
		}
		*out = unsafe.Pointer(self)
		return 0
	}),
	syscall.NewCallback(func(*browserSelectionHandler) uintptr { return 2 }),
	syscall.NewCallback(func(*browserSelectionHandler) uintptr { return 1 }),
	syscall.NewCallback(func(self *browserSelectionHandler, hr uintptr, result *uint16) uintptr {
		var selection string
		if int32(hr) >= 0 && result != nil {
			// UTF-16 JSON can use six code units for each escaped character.
			for length := 0; length <= 24002; length++ {
				buffer := unsafe.Slice(result, length+1)
				if buffer[length] == 0 {
					_ = json.Unmarshal([]byte(windows.UTF16ToString(buffer)), &selection)
					break
				}
			}
		}
		if complete := self.complete; complete != nil {
			self.complete = nil
			complete(strings.TrimSpace(selection))
		}
		return 0
	}),
}

func runFixedBrowserOperation(view *edge.ICoreWebView2, source string, handler *browserSelectionHandler) error {
	script, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	vtable := *(**[61]uintptr)(unsafe.Pointer(view))
	hr, _, _ := syscall.SyscallN(vtable[executeScript], uintptr(unsafe.Pointer(view)), uintptr(unsafe.Pointer(script)), uintptr(unsafe.Pointer(handler)))
	runtime.KeepAlive(script)
	if int32(hr) < 0 {
		return windows.Errno(hr)
	}
	return nil
}

func prefillBrandOrganization(page, organization string) string {
	// JSON escapes executable delimiters, including quotes, backslashes, and
	// line separators. Both strings are data for this fixed operation.
	values, _ := json.Marshal([2]string{page, organization})
	return `(([page, organization]) => {
  if (location.href !== page || location.protocol !== "https:") return "";
  const fields = Array.from(document.querySelectorAll('form input[autocomplete="organization"]'))
    .filter((field) => field.type === "text" && !field.matches(":disabled, [readonly]") &&
      field.value === "" && field.getClientRects().length &&
      (field.maxLength < 0 || field.maxLength >= organization.length));
  if (fields.length !== 1) return "";
  fields[0].value = organization;
  return "filled";
})(` + string(values) + `)`
}

type browserEvent struct {
	vtable *[4]uintptr
	invoke func(unsafe.Pointer) uintptr
}

// Event objects stay owned by the launcher until their handlers are removed.
var browserEventVtable = [4]uintptr{
	syscall.NewCallback(func(self *browserEvent, iid *windows.GUID, out *unsafe.Pointer) uintptr {
		if out == nil {
			return 0x80004003
		}
		*out = nil
		if iid == nil || *iid != (windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}) {
			return 0x80004002
		}
		*out = unsafe.Pointer(self)
		return 0
	}),
	syscall.NewCallback(func(*browserEvent) uintptr { return 2 }),
	syscall.NewCallback(func(*browserEvent) uintptr { return 1 }),
	syscall.NewCallback(func(self *browserEvent, sender, args unsafe.Pointer) uintptr {
		return self.invoke(args)
	}),
}

func subscribeBrowserEvent(view *edge.ICoreWebView2, slot int, callback func(unsafe.Pointer) uintptr) (func(), error) {
	handler := &browserEvent{&browserEventVtable, callback}
	vtable := *(**[61]uintptr)(unsafe.Pointer(view))
	var token int64
	hr, _, _ := syscall.SyscallN(vtable[slot], uintptr(unsafe.Pointer(view)), uintptr(unsafe.Pointer(handler)), uintptr(unsafe.Pointer(&token)))
	runtime.KeepAlive(handler)
	if int32(hr) < 0 {
		return nil, fmt.Errorf("register WebView2 navigation policy: %w", windows.Errno(hr))
	}
	return func() {
		_, _, _ = syscall.SyscallN(vtable[slot+1], uintptr(unsafe.Pointer(view)), uintptr(token))
		runtime.KeepAlive(handler)
	}, nil
}

func guardBrowser(view *edge.ICoreWebView2, allows func(string) bool, allowFrames bool, onNavigate func()) (func(), error) {
	nav := func(args unsafe.Pointer) uintptr {
		if args == nil {
			return 0x80004003
		}
		vtable := *(**[9]uintptr)(args)
		var uri *uint16
		hr, _, _ := syscall.SyscallN(vtable[navigationURI], uintptr(args), uintptr(unsafe.Pointer(&uri)))
		allowed := false
		if int32(hr) >= 0 && uri != nil {
			allowed = allows(windows.UTF16PtrToString(uri))
		}
		if uri != nil {
			_, _, _ = procCoTaskMemFree.Call(uintptr(unsafe.Pointer(uri)))
		}
		if allowed {
			return 0
		}
		hr, _, _ = syscall.SyscallN(vtable[navigationCancel], uintptr(args), 1)
		return hr
	}
	frame := func(args unsafe.Pointer) uintptr {
		if allowFrames {
			return nav(args)
		}
		if args == nil {
			return 0x80004003
		}
		hr, _, _ := syscall.SyscallN((*(**[9]uintptr)(args))[navigationCancel], uintptr(args), 1)
		return hr
	}
	top := func(args unsafe.Pointer) uintptr {
		if onNavigate != nil {
			onNavigate()
		}
		return nav(args)
	}
	popup := func(args unsafe.Pointer) uintptr {
		if args == nil {
			return 0x80004003
		}
		hr, _, _ := syscall.SyscallN((*(**[7]uintptr)(args))[newWindowHandled], uintptr(args), 1)
		return hr
	}
	var cleanup []func()
	closeGuards := func() {
		for _, f := range cleanup {
			f()
		}
	}
	for _, event := range []struct {
		slot   int
		invoke func(unsafe.Pointer) uintptr
	}{{addNavigationStarting, top}, {addFrameNavigationStarting, frame}, {addNewWindowRequested, popup}} {
		closeEvent, err := subscribeBrowserEvent(view, event.slot, event.invoke)
		if err != nil {
			closeGuards()
			return nil, err
		}
		cleanup = append(cleanup, closeEvent)
	}
	return closeGuards, nil
}

func isolateBrowser(chromium *edge.Chromium, allowsNavigation, allowsResource func(string) bool, research bool, onNavigate func()) (func(), error) {
	settings, err := chromium.GetSettings()
	if err != nil {
		return nil, err
	}
	defer settings.Release()
	if err := settings.PutAreHostObjectsAllowed(false); err != nil {
		return nil, err
	}
	if err := settings.PutIsWebMessageEnabled(!research); err != nil {
		return nil, err
	}
	view, err := chromium.GetController().GetCoreWebView2()
	if err != nil {
		return nil, err
	}
	defer view.Release()
	cleanup, err := guardBrowser(view, allowsNavigation, research, onNavigate)
	if err != nil {
		return nil, err
	}
	env := chromium.Environment()
	if env == nil {
		cleanup()
		return nil, errors.New("isolated WebView2 environment is unavailable")
	}
	blocked, err := env.CreateWebResourceResponse(nil, 403, "GRAB-E402", "Cache-Control: no-store")
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("prepare WebView2 origin rejection: %w", err)
	}
	removeRequestHandler, err := subscribeBrowserEvent(view, addWebResourceRequested, func(raw unsafe.Pointer) uintptr {
		if raw == nil {
			return 0x80004003
		}
		args := (*edge.ICoreWebView2WebResourceRequestedEventArgs)(raw)
		req, err := args.GetRequest()
		if err == nil && req != nil {
			defer req.Release()
			if uri, err := req.GetUri(); err == nil && allowsResource(uri) {
				var context int32
				vtable := *(**[8]uintptr)(raw)
				hr, _, _ := syscall.SyscallN(vtable[7], uintptr(raw), uintptr(unsafe.Pointer(&context)))
				if int32(hr) >= 0 && (context != edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT || allowsNavigation(uri)) {
					return 0
				}
			}
		}
		if err := args.PutResponse(blocked); err != nil {
			return 0x80004005
		}
		return 0
	})
	if err != nil {
		blocked.Release()
		cleanup()
		return nil, err
	}
	removeFilter, err := filterAllBrowserRequests(view)
	if err != nil {
		removeRequestHandler()
		blocked.Release()
		cleanup()
		return nil, err
	}
	if err := view.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL); err != nil {
		removeFilter()
		removeRequestHandler()
		blocked.Release()
		cleanup()
		return nil, err
	}
	return func() {
		pattern := [...]uint16{'*', 0}
		vtable := *(**[61]uintptr)(unsafe.Pointer(view))
		_, _, _ = syscall.SyscallN(vtable[removeWebResourceFilter], uintptr(unsafe.Pointer(view)), uintptr(unsafe.Pointer(&pattern[0])), edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL)
		runtime.KeepAlive(&pattern)
		removeFilter()
		removeRequestHandler()
		cleanup()
		blocked.Release()
	}, nil
}
