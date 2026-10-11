using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Security.Principal;
using Microsoft.Win32.SafeHandles;

// Only the pinned signed driver is loaded. The service is demand-start, never
// boot-start. An unrelated installed PawnIO service is never stopped or changed.
sealed class DriverSession : IDisposable
{
    const string ServiceName = "MosaicSensors-PawnIO-2-1";
    const string Device = @"\\?\GLOBALROOT\Device\PawnIO";
    public string Mode { get; private set; } = "unavailable";
    public string? Problem { get; private set; }
    public int Error { get; private set; }
    string? ownedPath;
    bool disposed;
    SafeFileHandle? devicePin;
    FileStream? readerLease;

    public static SafeFileHandle OpenDevice() => Native.CreateFile(Device, 0xC0000000, 3, IntPtr.Zero, 3, 0, IntPtr.Zero);

    public static DriverSession Open()
    {
        var session = new DriverSession();
        try
        {
            session.Acquire();
            if (session.Mode != "unavailable") session.devicePin = OpenDevice();
        }
        catch (Exception e)
        {
            session.Error = e is Win32Exception w ? w.NativeErrorCode : -1;
            session.Problem = "CPU sensor driver: " + e.Message;
        }
        return session;
    }

    void Acquire()
    {
        using var identity = WindowsIdentity.GetCurrent();
        bool admin = new WindowsPrincipal(identity).IsInRole(WindowsBuiltInRole.Administrator);
        using var device = OpenDevice();
        int deviceError = device.IsInvalid ? Marshal.GetLastWin32Error() : 0;
        if (!admin)
        {
            if (!device.IsInvalid) { Mode = "existing"; return; }
            Error = deviceError;
            Problem = "CPU temperature/power: run mosaic --sensors-admin to use the bundled session driver.";
            return;
        }
        using var gate = Lock();
        using var scm = Native.OpenSCManager(null, null, 3); // connect + create
        Check(scm);
        using var existing = Native.OpenService(scm, ServiceName, 0x25); // query config/status + stop
        if (!device.IsInvalid)
        {
            Mode = "existing";
            if (!existing.IsInvalid && IsOurs(existing, out var path))
            {
                ownedPath = path;
                Mode = "portable-session";
                AddReaderLease();
            }
            return;
        }
        // Access denied is not absence. Never collide with or replace another monitor.
        if (deviceError != 2 && deviceError != 3)
            throw new Win32Exception(deviceError);
        if (!existing.IsInvalid)
        {
            if (!IsOurs(existing, out var oldPath))
                throw new InvalidOperationException("The Mosaic sensor service name is already in use by a different configuration.");
            // Recover only our own stopped service after an interrupted session.
            if (!Native.QueryServiceStatus(existing, out var status)) throw LastError();
            if (status.State != 1) throw new InvalidOperationException("A previous sensor session is still closing. Try again shortly.");
            using var removal = Native.OpenService(scm, ServiceName, 0x10000);
            Check(removal);
            if (!Native.DeleteService(removal)) throw LastError();
            existing.Dispose();
            removal.Dispose();
            DeleteFiles(oldPath);
        }
        else if (Marshal.GetLastWin32Error() != 1060) throw LastError();
        if (!OperatingSystem.IsWindowsVersionAtLeast(10, 0, 19041))
            throw new PlatformNotSupportedException("Bundled CPU sensors require Windows 10 2004 or newer.");
        string source = Path.Combine(AppContext.BaseDirectory, "driver", "PawnIO.sys");
        byte[] bytes = File.ReadAllBytes(source);
        Verify(bytes);
        string directory = SecureDirectory();
        ownedPath = Path.Combine(directory, "PawnIO.sys");
        try
        {
            using (var file = new FileStream(ownedPath, FileMode.CreateNew, FileAccess.Write, FileShare.None)) file.Write(bytes);
            using var service = Native.CreateService(scm, ServiceName, "Mosaic temporary CPU sensors", 0x10034,
                1, 3, 1, ownedPath, null, IntPtr.Zero, null, null, null);
            Check(service);
            if (!Native.StartService(service, 0, IntPtr.Zero))
            {
                var error = LastError();
                Native.DeleteService(service);
                throw error;
            }
            Mode = "portable-session";
            AddReaderLease();
        }
        catch
        {
            DeleteFiles(ownedPath);
            ownedPath = null;
            throw;
        }
    }

    void AddReaderLease()
    {
        string path = Path.Combine(Path.GetDirectoryName(ownedPath!)!, $"reader-{Environment.ProcessId}-{Guid.NewGuid():N}.lease");
        readerLease = new FileStream(path, FileMode.CreateNew, FileAccess.ReadWrite,
            FileShare.None, 1, FileOptions.DeleteOnClose);
    }

    static bool HasOtherReaders(string driverPath)
    {
        foreach (string path in Directory.EnumerateFiles(Path.GetDirectoryName(driverPath)!, "reader-*.lease"))
        {
            try
            {
                // Active leases are held without sharing. DeleteOnClose also
                // releases leases if their process is terminated unexpectedly.
                using var stale = new FileStream(path, FileMode.Open, FileAccess.ReadWrite, FileShare.None, 1, FileOptions.DeleteOnClose);
            }
            catch (FileNotFoundException) { }
            catch (IOException) { return true; }
            catch (UnauthorizedAccessException) { return true; }
        }
        return false;
    }

    static void Verify(byte[] bytes)
    {
        string expected = RuntimeInformation.OSArchitecture switch
        {
            Architecture.X64 => "f92de04e5a02256e86ffa1b4252fe669282198b7329227f40e51bca9267d133d",
            Architecture.Arm64 => "2b663b621004c1646726aafffd0e2feffa468f6c6b8053ffb7f6995c436d797c",
            _ => throw new PlatformNotSupportedException("No bundled driver for this Windows architecture.")
        };
        if (!Convert.ToHexString(SHA256.HashData(bytes)).Equals(expected, StringComparison.OrdinalIgnoreCase))
            throw new InvalidDataException("Bundled driver checksum mismatch; extract a fresh Mosaic release.");
    }

    static bool IsOurs(ServiceHandle service, out string path)
    {
        path = "";
        Native.QueryServiceConfig(service, IntPtr.Zero, 0, out uint size);
        if (size == 0 || size > 65536) return false;
        IntPtr memory = Marshal.AllocHGlobal((int)size);
        try
        {
            if (!Native.QueryServiceConfig(service, memory, size, out _)) return false;
            var config = Marshal.PtrToStructure<ServiceConfig>(memory);
            path = Marshal.PtrToStringUni(config.Path) ?? "";
            if (config.Type != 1 || config.Start != 3 || Path.GetFileName(path) != "PawnIO.sys" ||
                !Path.GetFileName(Path.GetDirectoryName(path)!).StartsWith("MosaicSensors-", StringComparison.Ordinal)) return false;
            Verify(File.ReadAllBytes(path));
            return true;
        }
        catch { return false; }
        finally { Marshal.FreeHGlobal(memory); }
    }

    static string SecureDirectory()
    {
        // A fresh protected DACL prevents non-elevated replacement of the image.
        string path = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "MosaicSensors-" + Guid.NewGuid().ToString("N"));
        if (!Native.ConvertStringSecurityDescriptorToSecurityDescriptor("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", 1, out var sd, out _)) throw LastError();
        try
        {
            var security = new SecurityAttributes { Length = Marshal.SizeOf<SecurityAttributes>(), Descriptor = sd };
            if (!Native.CreateDirectory(path, ref security)) throw LastError();
        }
        finally { Native.LocalFree(sd); }
        return path;
    }

    sealed class Gate : IDisposable
    {
        readonly Mutex mutex = new(false, @"Global\MosaicSensors-PawnIO-2-1");
        public Gate()
        {
            try { if (!mutex.WaitOne(TimeSpan.FromSeconds(3))) { mutex.Dispose(); throw new TimeoutException("Another sensor session is busy."); } }
            catch (AbandonedMutexException) { }
        }
        public void Dispose() { mutex.ReleaseMutex(); mutex.Dispose(); }
    }
    static Gate Lock() => new();
    static void Check(ServiceHandle handle) { if (handle.IsInvalid) throw LastError(); }
    static Win32Exception LastError() => new(Marshal.GetLastWin32Error());
    static void DeleteFiles(string path)
    {
        try { File.Delete(path); Directory.Delete(Path.GetDirectoryName(path)!); }
        catch (IOException) { }
        catch (UnauthorizedAccessException) { }
    }

    public void Dispose()
    {
        if (disposed) return;
        disposed = true;
        devicePin?.Dispose();
        if (ownedPath == null) return;
        try
        {
            using var gate = Lock();
            readerLease?.Dispose();
            if (HasOtherReaders(ownedPath)) return;
            using var scm = Native.OpenSCManager(null, null, 1);
            Check(scm);
            using var service = Native.OpenService(scm, ServiceName, 0x10025);
            if (service.IsInvalid || !IsOurs(service, out var path) || path != ownedPath) return;
            // Only the last Mosaic lease may request unload. Open kernel handles
            // alone do not prevent a stop request from marking the device deleted.
            if (!Native.ControlService(service, 1, out _) && Marshal.GetLastWin32Error() != 1062)
            {
                Console.Error.WriteLine($"Sensor session remains in use (Windows error {Marshal.GetLastWin32Error()}).");
                return;
            }
            for (int i = 0; i < 20; i++)
            {
                if (!Native.QueryServiceStatus(service, out var status)) return;
                if (status.State == 1) break;
                if (i == 19) return;
                Thread.Sleep(50);
            }
            if (Native.DeleteService(service)) DeleteFiles(path);
        }
        catch (Exception e) { Console.Error.WriteLine("Sensor session cleanup: " + e.Message); }
        finally { readerLease?.Dispose(); }
    }

    [StructLayout(LayoutKind.Sequential)] struct SecurityAttributes { public int Length; public IntPtr Descriptor; public int Inherit; }
    [StructLayout(LayoutKind.Sequential)] struct ServiceConfig { public uint Type, Start, Error; public IntPtr Path, Group; public uint Tag; public IntPtr Dependencies, Account, Display; }
    [StructLayout(LayoutKind.Sequential)] internal struct ServiceStatus { public uint Type, State, Controls, Win32Exit, SpecificExit, Checkpoint, WaitHint; }
    internal sealed class ServiceHandle : SafeHandleZeroOrMinusOneIsInvalid
    {
        public ServiceHandle() : base(true) { }
        protected override bool ReleaseHandle() => Native.CloseServiceHandle(handle);
    }
    static class Native
    {
        [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern SafeFileHandle CreateFile(string path,uint access,uint share,IntPtr sa,uint creation,uint flags,IntPtr template);
        [DllImport("advapi32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern ServiceHandle OpenSCManager(string? machine,string? database,uint access);
        [DllImport("advapi32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern ServiceHandle OpenService(ServiceHandle manager,string name,uint access);
        [DllImport("advapi32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern ServiceHandle CreateService(ServiceHandle manager,string name,string display,uint access,uint type,uint start,uint error,string binary,string? group,IntPtr tag,string? dependencies,string? account,string? password);
        [DllImport("advapi32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool StartService(ServiceHandle service,uint count,IntPtr args);
        [DllImport("advapi32.dll", SetLastError=true)] public static extern bool ControlService(ServiceHandle service,uint control,out ServiceStatus status);
        [DllImport("advapi32.dll", SetLastError=true)] public static extern bool QueryServiceStatus(ServiceHandle service,out ServiceStatus status);
        [DllImport("advapi32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool QueryServiceConfig(ServiceHandle service,IntPtr config,uint size,out uint needed);
        [DllImport("advapi32.dll", SetLastError=true)] public static extern bool DeleteService(ServiceHandle service);
        [DllImport("advapi32.dll")] public static extern bool CloseServiceHandle(IntPtr service);
        [DllImport("advapi32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool ConvertStringSecurityDescriptorToSecurityDescriptor(string sddl,uint revision,out IntPtr descriptor,out uint size);
        [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool CreateDirectory(string path,ref SecurityAttributes attributes);
        [DllImport("kernel32.dll")] public static extern IntPtr LocalFree(IntPtr memory);
    }
}
