//go:build windows

package main

import "syscall"

func setConsoleUTF8() {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	_, _, _ = k32.NewProc("SetConsoleOutputCP").Call(uintptr(65001))
	_, _, _ = k32.NewProc("SetConsoleCP").Call(uintptr(65001))
}
