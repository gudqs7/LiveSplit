package main

import "golang.org/x/sys/windows"

func openURLWithShell(url string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, _ := windows.UTF16PtrFromString(url)
	return windows.ShellExecute(0, verb, file, nil, nil, swShowNormal)
}
