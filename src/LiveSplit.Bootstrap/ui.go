package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

const (
	bootstrapWindowClassName = "LiveSplit.Bootstrap.Window"
	wmBootstrapSynchronize   = win.WM_APP + 1
	errorClassAlreadyExists  = 1410

	primaryButtonID   = 1001
	secondaryButtonID = 1002
	closeButtonID     = 1003

	titleLabelID  = 1101
	bodyLabelID   = 1102
	statusLabelID = 1103
	progressID    = 1104
)

type setupMode int

const (
	setupModeInitial setupMode = iota
	setupModeBusy
	setupModeError
	setupModeRestart
)

type nativeSetupWindow struct {
	hwnd              win.HWND
	strings           bootstrapStrings
	managedExecutable string
	managedArguments  []string
	titleLabel        win.HWND
	bodyLabel         win.HWND
	statusLabel       win.HWND
	progressBar       win.HWND
	primaryButton     win.HWND
	secondaryButton   win.HWND
	closeButton       win.HWND
	titleFont         win.HFONT
	bodyFont          win.HFONT
	backgroundBrush   win.HBRUSH
	mu                sync.Mutex
	pendingActions    []func()
	closed            bool
	mode              setupMode
	cancelCurrent     context.CancelFunc
}

var (
	activeSetupWindow  *nativeSetupWindow
	nativeWndProc      = syscall.NewCallback(bootstrapWndProc)
	user32             = windows.NewLazySystemDLL("user32.dll")
	setProcessDPIAware = user32.NewProc("SetProcessDPIAware")
)

func newSetupWindow(strings bootstrapStrings, managedExecutable string, managedArguments []string) (*nativeSetupWindow, error) {
	return &nativeSetupWindow{
		strings:           strings,
		managedExecutable: managedExecutable,
		managedArguments:  managedArguments,
		mode:              setupModeInitial,
	}, nil
}

func (window *nativeSetupWindow) Run() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := window.create(); err != nil {
		return err
	}
	defer window.dispose()

	win.ShowWindow(window.hwnd, win.SW_SHOW)
	win.UpdateWindow(window.hwnd)
	win.SetForegroundWindow(window.hwnd)
	win.SetFocus(window.primaryButton)

	var message win.MSG
	for {
		result := win.GetMessage(&message, 0, 0, 0)
		if result == 0 {
			return nil
		}
		if result == -1 {
			return errors.New("the Windows message loop failed")
		}

		win.TranslateMessage(&message)
		win.DispatchMessage(&message)
	}
}

func (window *nativeSetupWindow) create() error {
	setProcessDPIAware.Call()

	var controls win.INITCOMMONCONTROLSEX
	controls.DwSize = uint32(unsafe.Sizeof(controls))
	controls.DwICC = win.ICC_PROGRESS_CLASS
	win.InitCommonControlsEx(&controls)

	instance := win.GetModuleHandle(nil)
	className := utf16Ptr(bootstrapWindowClassName)
	window.backgroundBrush = win.GetSysColorBrush(win.COLOR_BTNFACE)

	windowClass := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   nativeWndProc,
		HInstance:     instance,
		HIcon:         win.LoadIcon(0, win.MAKEINTRESOURCE(uintptr(win.IDI_APPLICATION))),
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(uintptr(win.IDC_ARROW))),
		HbrBackground: window.backgroundBrush,
		LpszClassName: className,
	}

	if win.RegisterClassEx(&windowClass) == 0 && win.GetLastError() != errorClassAlreadyExists {
		return fmt.Errorf("RegisterClassEx failed: %d", win.GetLastError())
	}

	activeSetupWindow = window
	title := utf16Ptr(window.strings.title)
	style := uint32(win.WS_CAPTION | win.WS_SYSMENU | win.WS_MINIMIZEBOX)
	dpi := systemDPI()
	windowWidth := scaleForDPI(640, dpi)
	windowHeight := scaleForDPI(420, dpi)
	screenWidth := win.GetSystemMetrics(win.SM_CXSCREEN)
	screenHeight := win.GetSystemMetrics(win.SM_CYSCREEN)
	x := (screenWidth - windowWidth) / 2
	y := (screenHeight - windowHeight) / 2

	window.hwnd = win.CreateWindowEx(
		win.WS_EX_CONTROLPARENT,
		className,
		title,
		style,
		x,
		y,
		windowWidth,
		windowHeight,
		0,
		0,
		instance,
		nil)
	if window.hwnd == 0 {
		activeSetupWindow = nil
		return fmt.Errorf("CreateWindowEx failed: %d", win.GetLastError())
	}

	if err := window.createControls(dpi); err != nil {
		win.DestroyWindow(window.hwnd)
		activeSetupWindow = nil
		return err
	}

	return nil
}

func (window *nativeSetupWindow) createControls(dpi uint32) error {
	instance := win.GetModuleHandle(nil)
	window.titleFont = createUIFont(14, true, dpi)
	window.bodyFont = createUIFont(9, false, dpi)
	if window.titleFont == 0 || window.bodyFont == 0 {
		return errors.New("failed to create fonts")
	}

	window.titleLabel = createStaticControl(
		window.hwnd,
		instance,
		titleLabelID,
		window.strings.requiredTitle,
		scaleForDPI(22, dpi),
		scaleForDPI(18, dpi),
		scaleForDPI(556, dpi),
		scaleForDPI(32, dpi),
		window.titleFont)

	window.bodyLabel = createStaticControl(
		window.hwnd,
		instance,
		bodyLabelID,
		window.strings.requiredMessage,
		scaleForDPI(22, dpi),
		scaleForDPI(58, dpi),
		scaleForDPI(556, dpi),
		scaleForDPI(120, dpi),
		window.bodyFont)

	window.statusLabel = createStaticControl(
		window.hwnd,
		instance,
		statusLabelID,
		window.strings.readyStatus,
		scaleForDPI(22, dpi),
		scaleForDPI(185, dpi),
		scaleForDPI(556, dpi),
		scaleForDPI(58, dpi),
		window.bodyFont)

	window.progressBar = createControl(
		win.WS_CHILD|win.WS_VISIBLE|win.PBS_SMOOTH|win.PBS_MARQUEE,
		window.hwnd,
		instance,
		"msctls_progress32",
		"",
		progressID,
		0,
		scaleForDPI(22, dpi),
		scaleForDPI(252, dpi),
		scaleForDPI(556, dpi),
		scaleForDPI(20, dpi))
	win.SendMessage(window.progressBar, win.PBM_SETRANGE32, 0, 100)
	win.SendMessage(window.progressBar, win.PBM_SETPOS, 0, 0)
	win.SendMessage(window.progressBar, win.PBM_SETMARQUEE, 0, 0)

	window.closeButton = createButtonControl(
		window.hwnd,
		instance,
		closeButtonID,
		window.strings.close,
		scaleForDPI(164, dpi),
		scaleForDPI(310, dpi),
		scaleForDPI(90, dpi),
		scaleForDPI(32, dpi),
		false,
		window.bodyFont)
	win.ShowWindow(window.closeButton, win.SW_HIDE)

	window.secondaryButton = createButtonControl(
		window.hwnd,
		instance,
		secondaryButtonID,
		window.strings.notNow,
		scaleForDPI(264, dpi),
		scaleForDPI(310, dpi),
		scaleForDPI(140, dpi),
		scaleForDPI(32, dpi),
		false,
		window.bodyFont)

	window.primaryButton = createButtonControl(
		window.hwnd,
		instance,
		primaryButtonID,
		window.strings.downloadAndInstall,
		scaleForDPI(414, dpi),
		scaleForDPI(310, dpi),
		scaleForDPI(164, dpi),
		scaleForDPI(32, dpi),
		false,
		window.bodyFont)

	if window.titleLabel == 0 || window.bodyLabel == 0 || window.statusLabel == 0 ||
		window.progressBar == 0 || window.primaryButton == 0 ||
		window.secondaryButton == 0 || window.closeButton == 0 {
		return errors.New("failed to create one or more window controls")
	}

	return nil
}

func (window *nativeSetupWindow) onPrimaryClicked() {
	switch window.mode {
	case setupModeInitial, setupModeError:
		window.startSetup()
	case setupModeRestart:
		window.close()
	}
}

func (window *nativeSetupWindow) onSecondaryClicked() {
	switch window.mode {
	case setupModeInitial:
		window.close()
	case setupModeBusy:
		window.cancelSetup()
	case setupModeError:
		if err := openDownloadPage(); err != nil {
			window.setStatus(formatTemplate(window.strings.unknownError, err))
		}
	}
}

func (window *nativeSetupWindow) startSetup() {
	ctx, cancel := context.WithCancel(context.Background())

	window.mu.Lock()
	window.cancelCurrent = cancel
	window.mu.Unlock()

	window.mode = setupModeBusy
	win.EnableWindow(window.primaryButton, false)
	setControlText(window.secondaryButton, window.strings.cancel)
	win.EnableWindow(window.secondaryButton, true)
	win.ShowWindow(window.closeButton, win.SW_HIDE)
	setProgressMode(window.progressBar, false, 0)
	window.setStatus(window.strings.downloading)

	go window.runSetup(ctx)
}

func (window *nativeSetupWindow) cancelSetup() {
	window.mu.Lock()
	cancel := window.cancelCurrent
	window.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	win.EnableWindow(window.secondaryButton, false)
	window.setStatus(window.strings.cancel)
}

func (window *nativeSetupWindow) runSetup(ctx context.Context) {
	installerPath := filepath.Join(os.TempDir(), "LiveSplit", "NDP481-Web.exe")
	err := downloadInstaller(ctx, installerPath, window.updateDownloadProgress)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			window.synchronize(window.close)
			return
		}

		window.showError(window.strings.downloadFailedTitle, window.strings.downloadFailedMessage, "")
		return
	}

	window.synchronize(func() {
		setProgressMode(window.progressBar, false, 100)
		window.setStatus(window.strings.downloadComplete)
		win.EnableWindow(window.secondaryButton, false)
	})

	time.Sleep(250 * time.Millisecond)
	window.synchronize(func() {
		window.setStatus(window.strings.preparingInstall)
	})

	exitCode, installErr := runInstaller(installerPath, windows.Handle(window.hwnd), func() {
		window.synchronize(func() {
			setProgressMode(window.progressBar, true, 0)
			window.setStatus(window.strings.installing)
		})
	})
	if installErr != nil {
		if errors.Is(installErr, windows.ERROR_CANCELLED) {
			window.showError(window.strings.installFailedTitle, window.strings.permissionDenied, "")
			return
		}

		window.showError(window.strings.installFailedTitle, window.strings.installFailedMessage, installErr.Error())
		return
	}

	installed := waitForFrameworkInstallation(10 * time.Minute)
	if exitCode == 3010 || exitCode == 1641 {
		window.showRestartRequired()
		return
	}
	if !installed {
		detail := window.strings.installNotVerified
		if exitCode != 0 {
			detail = formatTemplate(window.strings.installerFailed, formatExitCode(exitCode))
		}
		window.showError(
			window.strings.installFailedTitle,
			window.strings.installFailedMessage,
			detail)
		return
	}

	window.synchronize(func() {
		setProgressMode(window.progressBar, false, 100)
		window.setStatus(window.strings.installComplete)
	})

	if err := launchManagedExecutable(window.managedExecutable, window.managedArguments); err != nil {
		window.showError(window.strings.managedMissingTitle, window.strings.managedMissingMessage, err.Error())
		return
	}

	window.synchronize(window.close)
}

func (window *nativeSetupWindow) updateDownloadProgress(progress downloadProgress) {
	window.synchronize(func() {
		if progress.Total > 0 {
			percentage := int(progress.Received * 100 / progress.Total)
			if percentage > 100 {
				percentage = 100
			}
			setProgressMode(window.progressBar, false, percentage)
			window.setStatus(formatTemplate(
				window.strings.downloadProgress,
				formatBytes(progress.Received),
				formatBytes(progress.Total),
				percentage))
			return
		}

		setProgressMode(window.progressBar, true, 0)
		window.setStatus(formatTemplate(window.strings.downloadProgressUnknown, formatBytes(progress.Received)))
	})
}

func (window *nativeSetupWindow) showError(title, message, detail string) {
	window.synchronize(func() {
		window.mode = setupModeError
		setControlText(window.titleLabel, title)
		setControlText(window.bodyLabel, message)
		setControlText(window.statusLabel, detail)
		setProgressMode(window.progressBar, false, 0)
		setControlText(window.primaryButton, window.strings.retry)
		win.EnableWindow(window.primaryButton, true)
		setControlText(window.secondaryButton, window.strings.openDownloadPage)
		win.EnableWindow(window.secondaryButton, true)
		win.ShowWindow(window.secondaryButton, win.SW_SHOW)
		win.ShowWindow(window.closeButton, win.SW_SHOW)
		win.SetFocus(window.primaryButton)
	})
}

func (window *nativeSetupWindow) showRestartRequired() {
	window.synchronize(func() {
		window.mode = setupModeRestart
		setControlText(window.titleLabel, window.strings.restartRequiredTitle)
		setControlText(window.bodyLabel, window.strings.restartRequiredMessage)
		setControlText(window.statusLabel, "")
		setProgressMode(window.progressBar, false, 100)
		setControlText(window.primaryButton, window.strings.close)
		win.EnableWindow(window.primaryButton, true)
		win.ShowWindow(window.secondaryButton, win.SW_HIDE)
		win.ShowWindow(window.closeButton, win.SW_HIDE)
		win.SetFocus(window.primaryButton)
	})
}

func (window *nativeSetupWindow) setStatus(text string) {
	setControlText(window.statusLabel, text)
}

func (window *nativeSetupWindow) synchronize(action func()) {
	window.mu.Lock()
	if window.closed {
		window.mu.Unlock()
		return
	}
	window.pendingActions = append(window.pendingActions, action)
	hwnd := window.hwnd
	window.mu.Unlock()

	if hwnd != 0 {
		win.PostMessage(hwnd, wmBootstrapSynchronize, 0, 0)
	}
}

func (window *nativeSetupWindow) runPendingActions() {
	window.mu.Lock()
	actions := window.pendingActions
	window.pendingActions = nil
	window.mu.Unlock()

	for _, action := range actions {
		action()
	}
}

func (window *nativeSetupWindow) markClosed() {
	window.mu.Lock()
	window.closed = true
	cancel := window.cancelCurrent
	window.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (window *nativeSetupWindow) close() {
	window.markClosed()
	win.PostMessage(window.hwnd, win.WM_CLOSE, 0, 0)
}

func (window *nativeSetupWindow) dispose() {
	window.markClosed()
	if window.titleFont != 0 {
		win.DeleteObject(win.HGDIOBJ(window.titleFont))
	}
	if window.bodyFont != 0 {
		win.DeleteObject(win.HGDIOBJ(window.bodyFont))
	}
	activeSetupWindow = nil
}

func bootstrapWndProc(hwnd win.HWND, message uint32, wParam, lParam uintptr) uintptr {
	window := activeSetupWindow
	if window == nil {
		return win.DefWindowProc(hwnd, message, wParam, lParam)
	}

	switch message {
	case win.WM_COMMAND:
		if win.HIWORD(uint32(wParam)) == win.BN_CLICKED {
			switch win.LOWORD(uint32(wParam)) {
			case primaryButtonID:
				window.onPrimaryClicked()
				return 0
			case secondaryButtonID:
				window.onSecondaryClicked()
				return 0
			case closeButtonID:
				window.close()
				return 0
			}
		}

	case wmBootstrapSynchronize:
		window.runPendingActions()
		return 0

	case win.WM_CTLCOLORSTATIC:
		win.SetBkMode(win.HDC(wParam), win.TRANSPARENT)
		return uintptr(window.backgroundBrush)

	case win.WM_CLOSE:
		window.markClosed()
		win.DestroyWindow(hwnd)
		return 0

	case win.WM_DESTROY:
		win.PostQuitMessage(0)
		return 0
	}

	return win.DefWindowProc(hwnd, message, wParam, lParam)
}

func createStaticControl(parent win.HWND, instance win.HINSTANCE, id int, text string, x, y, width, height int32, font win.HFONT) win.HWND {
	control := createControl(
		win.WS_CHILD|win.WS_VISIBLE|win.SS_LEFT|win.SS_NOPREFIX,
		parent,
		instance,
		"STATIC",
		text,
		id,
		0,
		x,
		y,
		width,
		height)
	if control != 0 {
		win.SendMessage(control, win.WM_SETFONT, uintptr(font), 1)
	}
	return control
}

func createButtonControl(parent win.HWND, instance win.HINSTANCE, id int, text string, x, y, width, height int32, isDefault bool, font win.HFONT) win.HWND {
	style := uint32(win.WS_CHILD | win.WS_VISIBLE | win.WS_TABSTOP | win.BS_PUSHBUTTON)
	if isDefault {
		style = uint32(win.WS_CHILD | win.WS_VISIBLE | win.WS_TABSTOP | win.BS_DEFPUSHBUTTON)
	}

	control := createControl(style, parent, instance, "BUTTON", text, id, 0, x, y, width, height)
	if control != 0 {
		win.SendMessage(control, win.WM_SETFONT, uintptr(font), 1)
	}
	return control
}

func createControl(style uint32, parent win.HWND, instance win.HINSTANCE, className, text string, id int, exStyle uint32, x, y, width, height int32) win.HWND {
	return win.CreateWindowEx(
		exStyle,
		utf16Ptr(className),
		utf16Ptr(text),
		style,
		x,
		y,
		width,
		height,
		parent,
		win.HMENU(uintptr(id)),
		instance,
		nil)
}

func createUIFont(pointSize int32, bold bool, dpi uint32) win.HFONT {
	height := -win.MulDiv(pointSize, int32(dpi), 72)
	weight := int32(win.FW_NORMAL)
	if bold {
		weight = win.FW_BOLD
	}

	logFont := win.LOGFONT{
		LfHeight:  height,
		LfWeight:  weight,
		LfCharSet: win.DEFAULT_CHARSET,
		LfQuality: win.CLEARTYPE_QUALITY,
	}
	copy(logFont.LfFaceName[:], windows.StringToUTF16("Segoe UI"))
	return win.CreateFontIndirect(&logFont)
}

func systemDPI() uint32 {
	hdc := win.GetDC(0)
	if hdc == 0 {
		return 96
	}
	defer win.ReleaseDC(0, hdc)
	return uint32(win.GetDeviceCaps(hdc, win.LOGPIXELSY))
}

func scaleForDPI(value int32, dpi uint32) int32 {
	return win.MulDiv(value, int32(dpi), 96)
}

func setControlText(control win.HWND, text string) {
	if control == 0 {
		return
	}
	win.SendMessage(control, win.WM_SETTEXT, 0, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func utf16Ptr(value string) *uint16 {
	pointer, _ := windows.UTF16PtrFromString(value)
	return pointer
}

func setProgressMode(progress win.HWND, marquee bool, value int) {
	if marquee {
		win.SendMessage(progress, win.PBM_SETMARQUEE, 1, 30)
		return
	}

	win.SendMessage(progress, win.PBM_SETMARQUEE, 0, 0)
	win.SendMessage(progress, win.PBM_SETPOS, uintptr(value), 0)
}

func waitForFrameworkInstallation(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isFrameworkInstalled() {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}

	return isFrameworkInstalled()
}

func formatExitCode(exitCode uint32) string {
	if exitCode == 0 {
		return "0"
	}

	return fmt.Sprintf("%d (0x%08X)", exitCode, exitCode)
}
