package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

const (
	wsThickFrame   = 0x00040000
	wsMinimizeBox  = 0x00020000
	wsMaximizeBox  = 0x00010000
	wsSysMenu      = 0x00080000
	wsClipChildren = 0x02000000
	wsPopup        = 0x80000000
	workbenchStyle = wsPopup | wsThickFrame | wsMinimizeBox | wsMaximizeBox | wsSysMenu | wsClipChildren

	wmGetMinMaxInfo = 0x0024
	wmNCHitTest     = 0x0084
	wmNCRButtonUp   = 0x00A5
	wmSysCommand    = 0x0112
	wmDPIChanged    = 0x02E0
	htCaption       = 2
	scSize          = 0xf000
	scMove          = 0xf010
	scMinimize      = 0xf020
	scMaximize      = 0xf030
	scClose         = 0xf060
	scRestore       = 0xf120
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	titleBarHeight  = 32
)

var (
	procIsZoomed                     = user32.NewProc("IsZoomed")
	procGetSystemMenu                = user32.NewProc("GetSystemMenu")
	procEnableMenuItem               = user32.NewProc("EnableMenuItem")
	procTrackPopupMenu               = user32.NewProc("TrackPopupMenu")
	procPostMessageW                 = user32.NewProc("PostMessageW")
	procSetWindowPos                 = user32.NewProc("SetWindowPos")
	procGetDpiForWindow              = user32.NewProc("GetDpiForWindow")
	procSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")
	procMonitorFromWindow            = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW              = user32.NewProc("GetMonitorInfoW")
	procAdjustWindowRectExForDpi     = user32.NewProc("AdjustWindowRectExForDpi")
)

type windowMinMaxInfo struct {
	reserved, maxSize, maxPosition, minTrack, maxTrack point
}

type monitorInfo struct {
	size          uint32
	monitor, work clientRect
	flags         uint32
}

func setWorkbenchMaxBounds(hwnd uintptr, info *windowMinMaxInfo) {
	const monitorDefaultToNearest = 2
	monitor, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	bounds := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&bounds))); ok == 0 {
		return
	}
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	frame := clientRect{}
	if ok, _, _ := procAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&frame)), workbenchStyle, 0, 0, dpi); ok == 0 {
		return
	}
	info.maxPosition = point{bounds.work.left - bounds.monitor.left + frame.left, bounds.work.top - bounds.monitor.top + frame.top}
	info.maxSize = point{bounds.work.right - bounds.work.left + frame.right - frame.left, bounds.work.bottom - bounds.work.top + frame.bottom - frame.top}
}

func enableWorkbenchTitleBar(settings *edge.ICoreWebViewSettings) error {
	// ICoreWebView2Settings9 extends Settings8. The setter follows the getter
	// in the inherited WebView2 vtable.
	const putIsNonClientRegionSupportEnabled = 38
	iid := windows.GUID{Data1: 0x0528a73b, Data2: 0xe92d, Data3: 0x49f4, Data4: [8]byte{0x92, 0x7a, 0xe5, 0x47, 0xdd, 0xda, 0xa3, 0x7d}}
	var extended unsafe.Pointer
	base := *(**[3]uintptr)(unsafe.Pointer(settings))
	hr, _, _ := syscall.SyscallN(base[0], uintptr(unsafe.Pointer(settings)), uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&extended)))
	if int32(hr) < 0 || extended == nil {
		return errors.New("WebView2 runtime does not support the Groxy Desk title bar")
	}
	vtable := *(**[putIsNonClientRegionSupportEnabled + 1]uintptr)(extended)
	defer syscall.SyscallN(vtable[2], uintptr(extended))
	hr, _, _ = syscall.SyscallN(vtable[putIsNonClientRegionSupportEnabled], uintptr(extended), 1)
	if int32(hr) < 0 {
		return fmt.Errorf("enable WebView2 title bar: %w", windows.Errno(hr))
	}
	return nil
}

func windowMaximized(hwnd uintptr) bool {
	maximized, _, _ := procIsZoomed.Call(hwnd)
	return maximized != 0
}

func postWindowCommand(hwnd, command uintptr) {
	_, _, _ = procPostMessageW.Call(hwnd, wmSysCommand, command, 0)
}

func showWindowSystemMenu(hwnd, packedPoint uintptr) {
	menu, _, _ := procGetSystemMenu.Call(hwnd, 0)
	if menu == 0 {
		return
	}
	maximized := windowMaximized(hwnd)
	for _, action := range []struct {
		command uintptr
		enabled bool
	}{{scRestore, maximized}, {scMove, !maximized}, {scSize, !maximized}, {scMaximize, !maximized}} {
		flags := uintptr(0) // MF_BYCOMMAND | MF_ENABLED
		if !action.enabled {
			flags = 0x0001 // MF_GRAYED
		}
		_, _, _ = procEnableMenuItem.Call(menu, action.command, flags)
	}
	x, y := int16(packedPoint), int16(packedPoint>>16)
	const menuFlags = 0x0100 | 0x0002 // TPM_RETURNCMD | TPM_RIGHTBUTTON
	command, _, _ := procTrackPopupMenu.Call(menu, menuFlags, uintptr(x), uintptr(y), 0, hwnd, 0)
	if command != 0 {
		postWindowCommand(hwnd, command)
	}
}

func applyWindowDPIChange(hwnd, lParam uintptr) {
	rect := *(**clientRect)(unsafe.Pointer(&lParam))
	_, _, _ = procSetWindowPos.Call(hwnd, 0, uintptr(rect.left), uintptr(rect.top),
		uintptr(rect.right-rect.left), uintptr(rect.bottom-rect.top), swpNoZOrder|swpNoActivate)
}
