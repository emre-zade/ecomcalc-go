//go:build windows
package main

import (
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procFindWindowW  = user32.NewProc("FindWindowW")
	procSetWindowPos = user32.NewProc("SetWindowPos")
)

const (
	wHwndTopmost   int32 = -1
	wHwndNoTopmost int32 = -2
	wSwpNosize           = 0x0001
	wSwpNomove           = 0x0002
)

func setWindowsAlwaysOnTop(title string, on bool) {
	ptrTitle, _ := syscall.UTF16PtrFromString(title)
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(ptrTitle)))
	if hwnd == 0 {
		return
	}
	var hwndInsertAfter int32 = wHwndNoTopmost
	if on {
		hwndInsertAfter = wHwndTopmost
	}
	procSetWindowPos.Call(
		hwnd,
		uintptr(hwndInsertAfter),
		0, 0, 0, 0,
		uintptr(wSwpNomove|wSwpNosize),
	)
}