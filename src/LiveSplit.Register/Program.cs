using LiveSplit.Options;
using System;
using System.Diagnostics;

namespace LiveSplit.Register;

public class Program
{
    private static void Main(string[] args)
    {
        try
        {
            if (!EventLog.SourceExists("LiveSplit"))
            {
                EventLog.CreateEventSource("LiveSplit", "Application");
            }
        }
        catch { }

        try
        {
            FiletypeRegistryHelper.RegisterFileFormats();
        }
        catch (Exception ex)
        {
            Log.Error(ex);
        }

        // Register the managed process, which hosts the embedded browser controls.
        try
        {
            if (!InternetExplorerBrowserEmulation.IsBrowserEmulationSet("LiveSplit.Managed.exe"))
            {
                InternetExplorerBrowserEmulation.SetBrowserEmulationVersion("LiveSplit.Managed.exe");
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex);
        }
    }
}
