package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	seeMaskNoCloseProcess = 0x00000040
	swShowNormal          = 1
	waitObject0           = 0x00000000
	waitTimeout           = 0x00000102
	maxInstallerWaitMS    = 20 * 60 * 1000
)

type shellExecuteInfo struct {
	size          uint32
	mask          uint32
	window        windows.Handle
	verb          *uint16
	file          *uint16
	parameters    *uint16
	directory     *uint16
	show          int32
	instance      windows.Handle
	idList        uintptr
	class         *uint16
	classKey      windows.Handle
	hotKey        uint32
	iconOrMonitor windows.Handle
	process       windows.Handle
}

var (
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW = shell32.NewProc("ShellExecuteExW")
)

func runInstaller(installerPath string, ownerWindow windows.Handle, onStarted func()) (uint32, error) {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(installerPath)
	parameters, _ := windows.UTF16PtrFromString("/passive /norestart")
	directory, _ := windows.UTF16PtrFromString(filepath.Dir(installerPath))

	info := shellExecuteInfo{
		mask:       seeMaskNoCloseProcess,
		window:     ownerWindow,
		verb:       verb,
		file:       file,
		parameters: parameters,
		directory:  directory,
		show:       swShowNormal,
	}
	info.size = uint32(unsafe.Sizeof(info))

	result, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) {
			return 0, callErr
		}
		return 0, fmt.Errorf("ShellExecuteExW failed: %w", callErr)
	}
	if info.process == 0 {
		return 0, errors.New("the installer did not provide a process handle")
	}
	defer windows.CloseHandle(info.process)

	if onStarted != nil {
		onStarted()
	}

	waitResult, err := windows.WaitForSingleObject(info.process, maxInstallerWaitMS)
	if err != nil {
		return 0, err
	}
	if waitResult == waitTimeout {
		return 0, errInstallerTimedOut
	}
	if waitResult != waitObject0 {
		return 0, fmt.Errorf("unexpected wait result: %d", waitResult)
	}

	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.process, &exitCode); err != nil {
		return 0, err
	}

	return exitCode, nil
}

func openDownloadPage() error {
	return openURLWithShell(downloadPageURL)
}
