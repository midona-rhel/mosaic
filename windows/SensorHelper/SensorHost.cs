using System.Diagnostics;

// The host owns the driver lifetime. The collector is a child so that even
// undisposed native handles in third-party libraries are gone before unload.
static class SensorHost
{
    public static async Task<int> Run(string[] args)
    {
        var start = new ProcessStartInfo(Environment.ProcessPath!)
        {
            UseShellExecute = false, CreateNoWindow = true,
            RedirectStandardInput = true, RedirectStandardOutput = true,
            RedirectStandardError = true
        };
        start.ArgumentList.Add("--collect");
        start.ArgumentList.Add(args[0]);
        using var child = Process.Start(start) ?? throw new IOException("Cannot start sensor collector.");
        var output = Pump(child.StandardOutput.BaseStream, Console.OpenStandardOutput());
        var errors = Pump(child.StandardError.BaseStream, Console.OpenStandardError());
        var input = args[0] == "--stdio" ? Input(child) : Task.CompletedTask;
        if (args[0] == "--once") child.StandardInput.Close();
        try
        {
            var exited = child.WaitForExitAsync();
            if (args[0] == "--stdio" && await Task.WhenAny(exited, input) == input)
            {
                // EOF means Mosaic exited. Give the library time to close,
                // then terminate only our own collector if it is stuck.
                if (await Task.WhenAny(exited, Task.Delay(2000)) != exited) child.Kill(entireProcessTree: true);
            }
            await exited;
            await Task.WhenAll(output, errors);
            return child.ExitCode;
        }
        finally
        {
            if (!child.HasExited) { child.Kill(entireProcessTree: true); await child.WaitForExitAsync(); }
        }
    }

    static async Task Input(Process child)
    {
        try { await Pump(Console.OpenStandardInput(), child.StandardInput.BaseStream); }
        catch (IOException) { }
        finally { child.StandardInput.Close(); }
    }

    static async Task Pump(Stream from, Stream to)
    {
        var buffer = new byte[4096];
        int count;
        while ((count = await from.ReadAsync(buffer)) > 0)
        {
            await to.WriteAsync(buffer.AsMemory(0, count));
            await to.FlushAsync();
        }
    }
}
