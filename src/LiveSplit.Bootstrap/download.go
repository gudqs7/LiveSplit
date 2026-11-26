package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type downloadProgress struct {
	Received int64
	Total    int64
}

type progressReporter func(downloadProgress)

func downloadInstaller(ctx context.Context, destination string, reporter progressReporter) error {
	return downloadFromURL(ctx, webInstallerURL, destination, reporter)
}

func downloadFromURL(ctx context.Context, url, destination string, reporter progressReporter) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	request.Header.Set("User-Agent", "LiveSplit-Bootstrap/1.0")
	client := &http.Client{
		Timeout: 15 * time.Minute,
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status: %s", response.Status)
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(destination), "NDP481-Web-*.tmp")
	if err != nil {
		return err
	}

	tempPath := tempFile.Name()
	removeTempFile := true
	defer func() {
		if removeTempFile {
			_ = os.Remove(tempPath)
		}
	}()

	total := response.ContentLength
	reportProgress := newProgressThrottle(reporter)
	buffer := make([]byte, 64*1024)
	var received int64

	for {
		if err := ctx.Err(); err != nil {
			_ = tempFile.Close()
			return err
		}

		count, readErr := response.Body.Read(buffer)
		if count > 0 {
			written, writeErr := tempFile.Write(buffer[:count])
			if writeErr != nil {
				_ = tempFile.Close()
				return writeErr
			}
			if written != count {
				_ = tempFile.Close()
				return io.ErrShortWrite
			}

			received += int64(count)
			reportProgress(downloadProgress{Received: received, Total: total})
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = tempFile.Close()
			return readErr
		}
	}

	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tempPath, destination); err != nil {
		return err
	}

	removeTempFile = false
	reportProgress(downloadProgress{Received: received, Total: total})
	return nil
}

func newProgressThrottle(reporter progressReporter) progressReporter {
	if reporter == nil {
		return func(downloadProgress) {}
	}

	lastReport := time.Time{}
	var lastReceived int64
	return func(progress downloadProgress) {
		finished := progress.Total > 0 && progress.Received >= progress.Total
		if !finished && time.Since(lastReport) < 100*time.Millisecond && progress.Received-lastReceived < 256*1024 {
			return
		}

		lastReport = time.Now()
		lastReceived = progress.Received
		reporter(progress)
	}
}
