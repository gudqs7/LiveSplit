package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const (
	englishLanguageCode = "en-US"
	chineseLanguageCode = "zh-CN"
)

type bootstrapStrings struct {
	title                   string
	requiredTitle           string
	requiredMessage         string
	readyStatus             string
	downloadAndInstall      string
	notNow                  string
	downloading             string
	downloadProgress        string
	downloadProgressUnknown string
	downloadComplete        string
	preparingInstall        string
	installing              string
	installComplete         string
	restartRequiredTitle    string
	restartRequiredMessage  string
	installFailedTitle      string
	installFailedMessage    string
	installerFailed         string
	installNotVerified      string
	permissionDenied        string
	downloadFailedTitle     string
	downloadFailedMessage   string
	retry                   string
	openDownloadPage        string
	close                   string
	cancel                  string
	managedMissingTitle     string
	managedMissingMessage   string
	unknownError            string
}

type localizationFile struct {
	Code string            `json:"code"`
	Keys map[string]string `json:"keys"`
}

type settingsFile struct {
	UILanguage string `xml:"UILanguage"`
}

var fallbackStrings = map[string]string{
	"bootstrap.framework.title":         ".NET Framework 4.8.1 Required",
	"bootstrap.downloadAndInstall":      "Download and Install",
	"bootstrap.notNow":                  "Not Now",
	"bootstrap.framework.message":       "LiveSplit requires Microsoft .NET Framework 4.8.1.\r\n\r\nWe can download and install it for you now. No browser is required. LiveSplit will start automatically when setup finishes.",
	"bootstrap.framework.statusReady":   "The web installer is about 1.4 MB. Installation may take a few minutes.",
	"bootstrap.downloading":             "Downloading the .NET Framework installer...",
	"bootstrap.downloadProgress":        "Downloading: {0} / {1} ({2}%)",
	"bootstrap.downloadProgressUnknown": "Downloading: {0}",
	"bootstrap.downloadComplete":        "Download complete. Starting the installer...",
	"bootstrap.preparingInstall":        "Preparing the .NET Framework installer...",
	"bootstrap.installing":              "Installing .NET Framework 4.8.1. Follow any Windows permission prompts.",
	"bootstrap.installComplete":         "Installation complete. Starting LiveSplit...",
	"bootstrap.restartRequired.title":   "Restart Required",
	"bootstrap.restartRequired.message": ".NET Framework 4.8.1 was installed, but Windows must be restarted before LiveSplit can start.\r\n\r\nRestart Windows, then open LiveSplit again.",
	"bootstrap.installFailed.title":     "Installation Did Not Complete",
	"bootstrap.installFailed.message":   "LiveSplit could not finish installing .NET Framework 4.8.1. You can retry or open Microsoft's download page to install it manually.",
	"bootstrap.installerFailed":         "The installer returned error code {0}.",
	"bootstrap.installNotVerified":      "Setup finished, but Windows still reports that .NET Framework 4.8.1 is not installed.",
	"bootstrap.permissionDenied":        "Windows administrator permission is required for installation. Select Yes when Windows asks, then retry.",
	"bootstrap.downloadFailed.title":    "Download Failed",
	"bootstrap.downloadFailed.message":  "The .NET Framework installer could not be downloaded. Check your network connection and try again.",
	"bootstrap.retry":                   "Retry",
	"bootstrap.openDownloadPage":        "Open Download Page",
	"bootstrap.close":                   "Close",
	"bootstrap.cancel":                  "Cancel",
	"bootstrap.managedMissing.title":    "LiveSplit Could Not Start",
	"bootstrap.managedMissing.message":  "LiveSplit.Managed.exe could not be found. Make sure every file was extracted, then try again.",
	"bootstrap.unknownError":            "An unexpected error occurred: {0}",
}

func loadBootstrapStrings(baseDirectory string) bootstrapStrings {
	languageCode := resolveLanguageCode(baseDirectory)
	keys := loadLocalizationKeys(baseDirectory, languageCode)
	for key, value := range fallbackStrings {
		if _, exists := keys[key]; !exists {
			keys[key] = value
		}
	}

	key := func(name string) string {
		return keys[name]
	}

	return bootstrapStrings{
		title:                   "LiveSplit",
		requiredTitle:           key("bootstrap.framework.title"),
		requiredMessage:         key("bootstrap.framework.message"),
		readyStatus:             key("bootstrap.framework.statusReady"),
		downloadAndInstall:      key("bootstrap.downloadAndInstall"),
		notNow:                  key("bootstrap.notNow"),
		downloading:             key("bootstrap.downloading"),
		downloadProgress:        key("bootstrap.downloadProgress"),
		downloadProgressUnknown: key("bootstrap.downloadProgressUnknown"),
		downloadComplete:        key("bootstrap.downloadComplete"),
		preparingInstall:        key("bootstrap.preparingInstall"),
		installing:              key("bootstrap.installing"),
		installComplete:         key("bootstrap.installComplete"),
		restartRequiredTitle:    key("bootstrap.restartRequired.title"),
		restartRequiredMessage:  key("bootstrap.restartRequired.message"),
		installFailedTitle:      key("bootstrap.installFailed.title"),
		installFailedMessage:    key("bootstrap.installFailed.message"),
		installerFailed:         key("bootstrap.installerFailed"),
		installNotVerified:      key("bootstrap.installNotVerified"),
		permissionDenied:        key("bootstrap.permissionDenied"),
		downloadFailedTitle:     key("bootstrap.downloadFailed.title"),
		downloadFailedMessage:   key("bootstrap.downloadFailed.message"),
		retry:                   key("bootstrap.retry"),
		openDownloadPage:        key("bootstrap.openDownloadPage"),
		close:                   key("bootstrap.close"),
		cancel:                  key("bootstrap.cancel"),
		managedMissingTitle:     key("bootstrap.managedMissing.title"),
		managedMissingMessage:   key("bootstrap.managedMissing.message"),
		unknownError:            key("bootstrap.unknownError"),
	}
}

func resolveLanguageCode(baseDirectory string) string {
	configuredLanguage := readConfiguredLanguage(baseDirectory)
	return resolveLanguageSetting(configuredLanguage, getUserDefaultUILanguage())
}

func resolveLanguageSetting(configuredLanguage string, uiLanguageID uint32) string {
	normalized := strings.ToLower(strings.TrimSpace(configuredLanguage))
	if normalized != "" && normalized != "auto" {
		switch {
		case strings.HasPrefix(normalized, "zh"):
			return chineseLanguageCode
		case strings.HasPrefix(normalized, "en"):
			return englishLanguageCode
		}
	}

	primaryLanguage := uiLanguageID & 0x3FF
	if primaryLanguage == 0x04 {
		return chineseLanguageCode
	}

	return englishLanguageCode
}

func readConfiguredLanguage(baseDirectory string) string {
	settingsPath := filepath.Join(baseDirectory, "settings.cfg")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return ""
	}

	var settings settingsFile
	if err := xml.Unmarshal(data, &settings); err != nil {
		return ""
	}

	return strings.TrimSpace(settings.UILanguage)
}

func getUserDefaultUILanguage() uint32 {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procedure := kernel32.NewProc("GetUserDefaultUILanguage")
	value, _, _ := procedure.Call()
	return uint32(value)
}

func loadLocalizationKeys(baseDirectory, languageCode string) map[string]string {
	candidates := []string{languageCode, englishLanguageCode}
	keys := make(map[string]string)

	for _, candidate := range candidates {
		candidateKeys := loadLocalizationFile(baseDirectory, candidate)
		if candidateKeys == nil {
			continue
		}

		for key, value := range candidateKeys {
			if _, exists := keys[key]; !exists {
				keys[key] = value
			}
		}
	}

	return keys
}

func loadLocalizationFile(baseDirectory, languageCode string) map[string]string {
	if languageCode == "" {
		return nil
	}

	path := filepath.Join(baseDirectory, "Localization", languageCode+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var localization localizationFile
	if err := json.Unmarshal(data, &localization); err != nil {
		return nil
	}

	return localization.Keys
}

func formatTemplate(template string, args ...any) string {
	result := template
	for index, value := range args {
		result = strings.ReplaceAll(result, fmt.Sprintf("{%d}", index), fmt.Sprint(value))
	}

	return result
}
