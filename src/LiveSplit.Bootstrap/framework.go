package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	minimumFrameworkRelease = 533320
	managedExecutableName   = "LiveSplit.Managed.exe"
	webInstallerURL         = "https://go.microsoft.com/fwlink/?LinkId=2203304"
	downloadPageURL         = "https://dotnet.microsoft.com/download/dotnet-framework/net481"
)

var (
	errManagedExecutableMissing = errors.New("LiveSplit.Managed.exe was not found")
	errInstallerTimedOut        = errors.New("the installer timed out")
)

func isSupportedFrameworkRelease(release uint32) bool {
	return release >= minimumFrameworkRelease
}

func isFrameworkInstalled() bool {
	return isSupportedFrameworkRelease(readFrameworkRelease())
}

func readFrameworkRelease() uint32 {
	access := uint32(registry.QUERY_VALUE)
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full`,
		access|registry.WOW64_64KEY)
	if err != nil {
		key, err = registry.OpenKey(
			registry.LOCAL_MACHINE,
			`SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full`,
			access)
		if err != nil {
			return 0
		}
	}
	defer key.Close()

	release, _, err := key.GetIntegerValue("Release")
	if err != nil {
		return 0
	}

	return uint32(release)
}

func launchManagedExecutable(executablePath string, args []string) error {
	info, err := os.Stat(executablePath)
	if err != nil || info.IsDir() {
		return fmt.Errorf("%w: %s", errManagedExecutableMissing, executablePath)
	}

	command := exec.Command(executablePath, args...)
	command.Dir = filepath.Dir(executablePath)
	command.Stdout = nil
	command.Stderr = nil
	command.Stdin = nil

	if err := command.Start(); err != nil {
		return err
	}

	return command.Process.Release()
}
