package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFrameworkReleaseDetectionUsesFourEightOneMinimum(t *testing.T) {
	tests := []struct {
		release uint32
		want    bool
	}{
		{release: 0, want: false},
		{release: 528449, want: false},
		{release: 533319, want: false},
		{release: 533320, want: true},
		{release: 533325, want: true},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.release), func(t *testing.T) {
			if got := isSupportedFrameworkRelease(test.release); got != test.want {
				t.Fatalf("isSupportedFrameworkRelease(%d) = %v, want %v", test.release, got, test.want)
			}
		})
	}
}

func TestLanguagePreferenceUsesSettingsBeforeWindowsLanguage(t *testing.T) {
	if got := resolveLanguageSetting("en-US", 0x0804); got != englishLanguageCode {
		t.Fatalf("configured English resolved to %q", got)
	}
	if got := resolveLanguageSetting("zh-CN", 0x0409); got != chineseLanguageCode {
		t.Fatalf("configured Chinese resolved to %q", got)
	}
}

func TestLanguagePreferenceFollowsChineseWindowsWhenAutomatic(t *testing.T) {
	if got := resolveLanguageSetting("", 0x0804); got != chineseLanguageCode {
		t.Fatalf("automatic Chinese resolved to %q", got)
	}
	if got := resolveLanguageSetting("auto", 0x0409); got != englishLanguageCode {
		t.Fatalf("automatic English resolved to %q", got)
	}
}

func TestBootstrapReadsLocalizedKeysAndConfiguredLanguage(t *testing.T) {
	baseDirectory := t.TempDir()
	localizationDirectory := filepath.Join(baseDirectory, "Localization")
	if err := os.Mkdir(localizationDirectory, 0o755); err != nil {
		t.Fatal(err)
	}

	jsonData := []byte(`{
  "code": "zh-CN",
  "keys": {
    "bootstrap.framework.title": "需要 .NET Framework 4.8.1",
    "bootstrap.downloadAndInstall": "下载并安装"
  }
}`)
	if err := os.WriteFile(filepath.Join(localizationDirectory, "zh-CN.json"), jsonData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(baseDirectory, "settings.cfg"),
		[]byte(`<Settings><UILanguage>zh-CN</UILanguage></Settings>`),
		0o644); err != nil {
		t.Fatal(err)
	}

	strings := loadBootstrapStrings(baseDirectory)
	if strings.requiredTitle != "需要 .NET Framework 4.8.1" {
		t.Fatalf("required title = %q", strings.requiredTitle)
	}
	if strings.downloadAndInstall != "下载并安装" {
		t.Fatalf("download button = %q", strings.downloadAndInstall)
	}
	if strings.retry != fallbackStrings["bootstrap.retry"] {
		t.Fatalf("fallback translation was not merged: %q", strings.retry)
	}
}

func TestLocalizationCatalogsContainBootstrapKeys(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate bootstrap test source")
	}

	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	for _, languageCode := range []string{englishLanguageCode, chineseLanguageCode} {
		t.Run(languageCode, func(t *testing.T) {
			path := filepath.Join(repositoryRoot, "src", "LiveSplit.Core", "Localization", "Locales", languageCode+".json")
			keys := loadLocalizationFileFromPath(path)
			for key := range fallbackStrings {
				if _, exists := keys[key]; !exists {
					t.Errorf("%s is missing %s", path, key)
				}
			}
		})
	}
}

func loadLocalizationFileFromPath(path string) map[string]string {
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

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{bytes: 512, want: "512 B"},
		{bytes: 1536, want: "1.5 KB"},
		{bytes: 1466664, want: "1.4 MB"},
		{bytes: 5 * 1024 * 1024 * 1024, want: "5.0 GB"},
	}

	for _, test := range tests {
		if got := formatBytes(test.bytes); got != test.want {
			t.Fatalf("formatBytes(%d) = %q, want %q", test.bytes, got, test.want)
		}
	}
}

func TestDownloadReportsProgressAndWritesFile(t *testing.T) {
	payload := []byte("installer payload")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = response.Write(payload)
	}))
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "installer.exe")
	var reports []downloadProgress
	if err := downloadFromURL(
		context.Background(),
		server.URL,
		destination,
		func(progress downloadProgress) {
			reports = append(reports, progress)
		}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Fatalf("downloaded %q, want %q", data, payload)
	}
	if len(reports) == 0 {
		t.Fatal("no progress was reported")
	}

	last := reports[len(reports)-1]
	if last.Received != int64(len(payload)) || last.Total != int64(len(payload)) {
		t.Fatalf("final progress = %+v", last)
	}
}
