using LiveSplit.Options;
using System.IO;
using Xunit;

namespace LiveSplit.Tests.Options;

public class FiletypeRegistryHelperMust
{
    [Theory]
    [InlineData("LiveSplit.Managed.exe")]
    [InlineData("LiveSplit.Register.exe")]
    [InlineData("LiveSplit.exe")]
    public void ResolveNativeLauncherPathFromAnyExecutableName(string executableName)
    {
        string directory = Path.Combine("C:\\", "LiveSplit");
        string executablePath = Path.Combine(directory, executableName);

        string launcherPath = FiletypeRegistryHelper.GetLauncherExecutablePath(executablePath);

        Assert.Equal(Path.Combine(directory, "LiveSplit.exe"), launcherPath);
    }
}
