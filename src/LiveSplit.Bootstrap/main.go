package main

import (
	"os"
	"path/filepath"

	"github.com/lxn/win"
)

func main() {
	baseDirectory, err := executableDirectory()
	if err != nil {
		showFatalError("LiveSplit", err)
		return
	}

	strings := loadBootstrapStrings(baseDirectory)
	managedExecutablePath := filepath.Join(baseDirectory, managedExecutableName)

	forceSetupUI := os.Getenv("LIVESPLIT_BOOTSTRAP_FORCE_SETUP") == "1"
	if !forceSetupUI && isFrameworkInstalled() {
		if err := launchManagedExecutable(managedExecutablePath, os.Args[1:]); err != nil {
			showMessage(strings.title, strings.managedMissingMessage)
		}

		return
	}

	window, err := newSetupWindow(strings, managedExecutablePath, os.Args[1:])
	if err != nil {
		showFatalError(strings.title, err)
		return
	}

	if err := window.Run(); err != nil {
		showFatalError(strings.title, err)
	}
}

func executableDirectory() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.Dir(executablePath), nil
}

func showFatalError(title string, err error) {
	showMessage(title, err.Error())
}

func showMessage(title, message string) {
	win.MessageBox(
		0,
		utf16Ptr(message),
		utf16Ptr(title),
		win.MB_OK|win.MB_ICONERROR)
}
