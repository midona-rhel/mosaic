using System.ComponentModel;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.Json;
using System.Security.Cryptography;
using LibreHardwareMonitor.Hardware;
using LibreHardwareMonitor.PawnIo;
using Microsoft.Win32.SafeHandles;

// A private stdin/stdout protocol. No listening socket, service or sensor writes.
Console.OutputEncoding = new UTF8Encoding(false);
if (args.SequenceEqual(new[] { "--setup" }))
{
    const string digest = "a3a46226c5e2824f4cdd42be0eecbabfc672c86f7889710f5ab1e6ad385b47a0";
    const string url = "https://raw.githubusercontent.com/LibreHardwareMonitor/LibreHardwareMonitor/v0.9.6/LibreHardwareMonitor/Resources/PawnIO_setup.exe";
    var directory = Path.Combine(Path.GetTempPath(), "Mosaic-sensors-" + Guid.NewGuid());
    Directory.CreateDirectory(directory);
    var installer = Path.Combine(directory, "PawnIO_setup.exe");
    try
    {
        Console.WriteLine("Downloading the pinned PawnIO installer. Windows will ask permission to install its sensor driver.");
        using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(60) };
        using var response = await http.GetAsync(url, HttpCompletionOption.ResponseHeadersRead);
        response.EnsureSuccessStatusCode();
        await using (var output = File.Create(installer))
        await using (var input = await response.Content.ReadAsStreamAsync())
        {
            byte[] buffer = new byte[65536]; int count; long total = 0;
            while ((count = await input.ReadAsync(buffer)) > 0)
            {
                total += count;
                if (total > 20 * 1024 * 1024) throw new IOException("Installer exceeds size limit.");
                await output.WriteAsync(buffer.AsMemory(0, count));
            }
        }
        var actual = Convert.ToHexString(SHA256.HashData(await File.ReadAllBytesAsync(installer)));
        if (!actual.Equals(digest, StringComparison.OrdinalIgnoreCase))
            throw new IOException("Installer checksum mismatch; installation was cancelled.");
        // The upstream installer owns the UAC prompt and installation UI.
        using var process = Process.Start(new ProcessStartInfo(installer, "-install")
        { UseShellExecute = true, Verb = "runas" });
        if (process == null) return 1;
        process.WaitForExit();
        return process.ExitCode;
    }
    catch (Win32Exception e) when (e.NativeErrorCode == 1223)
    { Console.Error.WriteLine("Sensor setup cancelled."); return 1; }
    catch (Exception e) { Console.Error.WriteLine("Sensor setup failed: " + e.Message); return 1; }
    finally { try { Directory.Delete(directory, true); } catch (IOException) { } }
}
if (!args.SequenceEqual(new[] { "--stdio" }) && !args.SequenceEqual(new[] { "--once" }))
{ Console.Error.WriteLine("Use --stdio, --once or --setup."); return 2; }

var computer = new Computer { IsCpuEnabled = true, IsGpuEnabled = true };
bool opened = false;
try
{
    while (args[0] == "--once" || Console.ReadLine() == "sample")
    {
        var report = new Report();
        report.DriverInstalled = PawnIo.IsInstalled;
        report.DriverVersion = PawnIo.Version?.ToString();
        using var driver = Native.CreateFile(@"\\?\GLOBALROOT\Device\PawnIO", 0xC0000000, 3,
            IntPtr.Zero, 3, 0, IntPtr.Zero);
        report.DriverAccessible = !driver.IsInvalid;
        report.DriverError = report.DriverAccessible ? 0 : Marshal.GetLastWin32Error();
        if (!report.DriverInstalled)
            report.Problems.Add("CPU sensors: run mosaic --setup-sensors to install PawnIO.");
        else if (!report.DriverAccessible)
            report.Problems.Add(report.DriverError == 5
                ? "CPU sensors: PawnIO access denied. Run mosaic --sensors-admin."
                : $"CPU sensors: PawnIO cannot be opened (Windows error {report.DriverError}). Run mosaic --setup-sensors to repair it.");
        try
        {
            if (!opened) { computer.Open(); opened = true; }
            foreach (var hardware in computer.Hardware) ReadHardware(hardware, report, null);
        }
        catch (Exception e) { report.Problems.Add($"Sensor library: {e.GetType().Name}: {e.Message}"); }
        Console.WriteLine(JsonSerializer.Serialize(report));
        if (args[0] == "--once") break;
    }
}
finally { computer.Close(); }
return 0;

static void ReadHardware(IHardware hardware, Report report, IHardware? root)
{
    root ??= hardware;
    try
    {
        hardware.Update();
        foreach (var sensor in hardware.Sensors)
        {
            var value = sensor.Value;
            if (value.HasValue && !float.IsFinite(value.Value)) value = null;
            // Without a working driver CPU register reads may become synthetic
            // zeros inside the library. Do not export those as measurements.
            if (root.HardwareType == HardwareType.Cpu && !report.DriverAccessible &&
                sensor.SensorType is SensorType.Temperature or SensorType.Power or SensorType.Clock) value = null;
            report.Sensors.Add(new SensorRow(sensor.Identifier.ToString(), hardware.Identifier.ToString(),
                root.Identifier.ToString(), root.Name, root.HardwareType.ToString(), sensor.Name,
                sensor.SensorType.ToString(), value));
        }
        foreach (var sub in hardware.SubHardware) ReadHardware(sub, report, root);
    }
    catch (Exception e) { report.Problems.Add($"{root.Name}: {e.GetType().Name}: {e.Message}"); }
}

record SensorRow(string ID, string Parent, string DeviceID, string Hardware, string HardwareType,
    string Name, string Type, float? Value)
{ public string Source => "LibreHardwareMonitorLib 0.9.6"; }
class Report
{
    public int Protocol => 1;
    public DateTimeOffset At => DateTimeOffset.UtcNow;
    public bool DriverInstalled { get; set; }
    public bool DriverAccessible { get; set; }
    public string? DriverVersion { get; set; }
    public int DriverError { get; set; }
    public List<SensorRow> Sensors { get; } = new();
    public List<string> Problems { get; } = new();
}
static class Native
{
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern SafeFileHandle CreateFile(string name, uint access, uint sharing,
        IntPtr security, uint creation, uint flags, IntPtr template);
}
