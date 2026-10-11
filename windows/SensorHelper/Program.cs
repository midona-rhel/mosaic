using System.Runtime.InteropServices;
using System.Text;
using System.Text.Json;
using LibreHardwareMonitor.Hardware;
using LibreHardwareMonitor.PawnIo;
using Microsoft.Win32.SafeHandles;

// A private stdin/stdout protocol. No listening socket or sensor writes.
Console.OutputEncoding = new UTF8Encoding(false);
bool collect = args.Length == 2 && args[0] == "--collect";
if (collect) args = args[1..];
if (!args.SequenceEqual(new[] { "--stdio" }) && !args.SequenceEqual(new[] { "--once" }))
{ Console.Error.WriteLine("Use --stdio or --once."); return 2; }

using var driverSession = DriverSession.Open();
if (!collect) return await SensorHost.Run(args);
var computer = new Computer { IsCpuEnabled = true, IsGpuEnabled = true };
bool opened = false;
try
{
    while (args[0] == "--once" || Console.ReadLine() == "sample")
    {
        var report = new Report { DriverMode = driverSession.Mode };
        if (driverSession.Problem != null) report.Problems.Add(driverSession.Problem);
        report.DriverInstalled = PawnIo.IsInstalled;
        report.DriverVersion = PawnIo.Version?.ToString();
        using var driver = Native.CreateFile(@"\\?\GLOBALROOT\Device\PawnIO", 0xC0000000, 3,
            IntPtr.Zero, 3, 0, IntPtr.Zero);
        report.DriverAccessible = !driver.IsInvalid;
        report.DriverError = report.DriverAccessible ? 0 : Marshal.GetLastWin32Error();
        if (report.DriverAccessible && driverSession.Mode == "portable-session") report.DriverVersion = "2.1.0";
        if (!report.DriverAccessible && driverSession.Problem == null)
            report.Problems.Add($"CPU sensor device unavailable (Windows error {report.DriverError}).");
        try
        {
            computer.IsCpuEnabled = report.DriverAccessible;
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
    public string DriverMode { get; set; } = "unavailable";
    public List<SensorRow> Sensors { get; } = new();
    public List<string> Problems { get; } = new();
}
static class Native
{
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    public static extern SafeFileHandle CreateFile(string name, uint access, uint sharing,
        IntPtr security, uint creation, uint flags, IntPtr template);
}
