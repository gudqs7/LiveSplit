# Native bootstrap

`LiveSplit.exe` is built from this directory as a self-contained native Go executable. It checks for .NET Framework 4.8.1, downloads Microsoft's web installer with progress reporting, runs setup, and then starts `LiveSplit.Managed.exe`.

The normal `dotnet build` already invokes `go build`. Run the focused tests with:

```powershell
go test ./...
```

After changing `LiveSplit.Bootstrap.manifest` or the application icon, regenerate the embedded Windows resources:

```powershell
go run github.com/akavel/rsrc@v0.10.2 -manifest LiveSplit.Bootstrap.manifest -ico ..\LiveSplit\Icon.ico -arch amd64 -o rsrc_windows_amd64.syso
```
