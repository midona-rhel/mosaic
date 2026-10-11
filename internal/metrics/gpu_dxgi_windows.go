//go:build windows

package metrics

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IDXGIFactory1::EnumAdapters1 and IDXGIAdapter1::GetDesc1 expose the same
// adapter LUID used in WDDM counters, without guessing from device order.
// https://learn.microsoft.com/windows/win32/api/dxgi/ns-dxgi-dxgi_adapter_desc1
type dxgiObject struct{ table *[13]uintptr }
type dxgiDescription struct {
	Name                                          [128]uint16
	Vendor, Device, Subsystem, Revision           uint32
	DedicatedVideo, DedicatedSystem, SharedSystem uintptr
	Low                                           uint32
	High                                          int32
	Flags                                         uint32
}

func windowsAdapters() ([]windowsAdapter, error) {
	create := windows.NewLazySystemDLL("dxgi.dll").NewProc("CreateDXGIFactory1")
	if err := create.Find(); err != nil {
		return nil, err
	}
	iid := windows.GUID{Data1: 0x770aae78, Data2: 0xf26f, Data3: 0x4dba, Data4: [8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}
	var factory *dxgiObject
	hr, _, _ := create.Call(uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&factory)))
	if int32(hr) < 0 {
		return nil, fmt.Errorf("CreateDXGIFactory1: HRESULT %#x", uint32(hr))
	}
	defer syscall.SyscallN(factory.table[2], uintptr(unsafe.Pointer(factory)))
	var result []windowsAdapter
	for index := uintptr(0); index < 64; index++ {
		var adapter *dxgiObject
		hr, _, _ = syscall.SyscallN(factory.table[12], uintptr(unsafe.Pointer(factory)), index, uintptr(unsafe.Pointer(&adapter)))
		if uint32(hr) == 0x887a0002 {
			break
		} // DXGI_ERROR_NOT_FOUND
		if int32(hr) < 0 {
			return nil, fmt.Errorf("EnumAdapters1: HRESULT %#x", uint32(hr))
		}
		var d dxgiDescription
		hr, _, _ = syscall.SyscallN(adapter.table[10], uintptr(unsafe.Pointer(adapter)), uintptr(unsafe.Pointer(&d)))
		syscall.SyscallN(adapter.table[2], uintptr(unsafe.Pointer(adapter)))
		if int32(hr) < 0 {
			continue
		}
		result = append(result, windowsAdapter{
			LUID: fmt.Sprintf("luid_0x%08x_0x%08x", uint32(d.High), d.Low),
			Name: windows.UTF16ToString(d.Name[:]), Vendor: d.Vendor,
			Memory: uint64(d.DedicatedVideo), Software: d.Flags&2 != 0,
		})
	}
	return result, nil
}
