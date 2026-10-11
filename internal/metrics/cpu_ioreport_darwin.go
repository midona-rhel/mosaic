//go:build darwin

package metrics

// Read-only IOReport adapter. Subscription, CLPC channel construction and units
// adapted from macmon (MIT); see THIRD_PARTY_NOTICES. Native handles are scoped
// to each sample window and released before returning; no background worker or
// persistent subscription survives a failed sample.
import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

var reportAPI struct {
	once                                                 sync.Once
	err                                                  error
	all                                                  func(uint64, uint64) uintptr
	subscribe                                            func(uintptr, uintptr, *uintptr, uint64, uintptr) uintptr
	sample                                               func(uintptr, uintptr, uintptr) uintptr
	delta                                                func(uintptr, uintptr, uintptr) uintptr
	group, subgroup, name, unit                          func(uintptr) uintptr
	driver, id                                           func(uintptr) uint64
	integer                                              func(uintptr, int32) int64
	release                                              func(uintptr)
	typeID                                               func(uintptr) uint64
	stringType, arrayType, dictType                      func() uint64
	stringNew                                            func(uintptr, string, uint32) uintptr
	stringRead                                           func(uintptr, unsafe.Pointer, int64, uint32) bool
	numberNew                                            func(uintptr, int32, unsafe.Pointer) uintptr
	arrayNew                                             func(uintptr, int64, uintptr) uintptr
	arrayAppend                                          func(uintptr, uintptr)
	arrayCount                                           func(uintptr) int64
	arrayGet                                             func(uintptr, int64) uintptr
	dictNew                                              func(uintptr, int64, uintptr, uintptr) uintptr
	dictCopy                                             func(uintptr, int64, uintptr) uintptr
	dictSet                                              func(uintptr, uintptr, uintptr)
	dictGet                                              func(uintptr, uintptr) uintptr
	registryID                                           func(uint32, *uint64) int32
	property                                             func(uint32, uintptr, uintptr, uint32) uintptr
	arrayCallbacks, dictKeyCallbacks, dictValueCallbacks uintptr
}

func initReportAPI() {
	smcAPI.once.Do(initSMCAPI)
	if smcAPI.err != nil {
		reportAPI.err = smcAPI.err
		return
	}
	for _, lib := range []struct {
		path    string
		symbols []struct {
			name string
			fn   any
		}
	}{
		{"/usr/lib/libIOReport.dylib", []struct {
			name string
			fn   any
		}{
			{"IOReportCopyAllChannels", &reportAPI.all}, {"IOReportCreateSubscription", &reportAPI.subscribe},
			{"IOReportCreateSamples", &reportAPI.sample}, {"IOReportCreateSamplesDelta", &reportAPI.delta},
			{"IOReportChannelGetGroup", &reportAPI.group}, {"IOReportChannelGetSubGroup", &reportAPI.subgroup},
			{"IOReportChannelGetChannelName", &reportAPI.name}, {"IOReportChannelGetUnitLabel", &reportAPI.unit},
			{"IOReportChannelGetDriverID", &reportAPI.driver}, {"IOReportChannelGetChannelID", &reportAPI.id},
			{"IOReportSimpleGetIntegerValue", &reportAPI.integer},
		}},
		{"/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", []struct {
			name string
			fn   any
		}{
			{"CFRelease", &reportAPI.release}, {"CFGetTypeID", &reportAPI.typeID},
			{"CFStringGetTypeID", &reportAPI.stringType}, {"CFArrayGetTypeID", &reportAPI.arrayType}, {"CFDictionaryGetTypeID", &reportAPI.dictType},
			{"CFStringCreateWithCString", &reportAPI.stringNew}, {"CFStringGetCString", &reportAPI.stringRead},
			{"CFNumberCreate", &reportAPI.numberNew}, {"CFArrayCreateMutable", &reportAPI.arrayNew}, {"CFArrayAppendValue", &reportAPI.arrayAppend},
			{"CFArrayGetCount", &reportAPI.arrayCount}, {"CFArrayGetValueAtIndex", &reportAPI.arrayGet},
			{"CFDictionaryCreateMutable", &reportAPI.dictNew}, {"CFDictionaryCreateMutableCopy", &reportAPI.dictCopy},
			{"CFDictionarySetValue", &reportAPI.dictSet}, {"CFDictionaryGetValue", &reportAPI.dictGet},
		}},
		{"/System/Library/Frameworks/IOKit.framework/IOKit", []struct {
			name string
			fn   any
		}{
			{"IORegistryEntryGetRegistryEntryID", &reportAPI.registryID}, {"IORegistryEntryCreateCFProperty", &reportAPI.property},
		}},
	} {
		handle, err := purego.Dlopen(lib.path, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			reportAPI.err = err
			return
		}
		for _, e := range lib.symbols {
			p, err := purego.Dlsym(handle, e.name)
			if err != nil {
				reportAPI.err = err
				return
			}
			purego.RegisterFunc(e.fn, p)
		}
		if strings.Contains(lib.path, "CoreFoundation") {
			for _, e := range []struct {
				name string
				p    *uintptr
			}{
				{"kCFTypeArrayCallBacks", &reportAPI.arrayCallbacks}, {"kCFTypeDictionaryKeyCallBacks", &reportAPI.dictKeyCallbacks}, {"kCFTypeDictionaryValueCallBacks", &reportAPI.dictValueCallbacks},
			} {
				p, err := purego.Dlsym(handle, e.name)
				if err != nil {
					reportAPI.err = err
					return
				}
				*e.p = p
			}
		}
	}
}
func cfRelease(p uintptr) {
	if p != 0 {
		reportAPI.release(p)
	}
}
func cfString(s string) uintptr { return reportAPI.stringNew(0, s, 0x08000100) }
func cfText(p uintptr) string {
	if p == 0 || reportAPI.typeID(p) != reportAPI.stringType() {
		return ""
	}
	var b [4096]byte
	if !reportAPI.stringRead(p, unsafe.Pointer(&b[0]), int64(len(b)), 0x08000100) {
		return ""
	}
	return cString(b[:])
}
func cfGet(dict uintptr, key string) uintptr {
	if dict == 0 || reportAPI.typeID(dict) != reportAPI.dictType() {
		return 0
	}
	k := cfString(key)
	if k == 0 {
		return 0
	}
	defer cfRelease(k)
	return reportAPI.dictGet(dict, k)
}
func cfSet(dict uintptr, key string, value uintptr) {
	k := cfString(key)
	defer cfRelease(k)
	if k != 0 && dict != 0 && value != 0 {
		reportAPI.dictSet(dict, k, value)
	}
}
func cfNumber(v int64) uintptr { return reportAPI.numberNew(0, 4, unsafe.Pointer(&v)) }
func cfDict() uintptr {
	return reportAPI.dictNew(0, 0, reportAPI.dictKeyCallbacks, reportAPI.dictValueCallbacks)
}
func cfArray() uintptr { return reportAPI.arrayNew(0, 0, reportAPI.arrayCallbacks) }
func reportChannels(dict uintptr) []uintptr {
	array := cfGet(dict, "IOReportChannels")
	if array == 0 || reportAPI.typeID(array) != reportAPI.arrayType() {
		return nil
	}
	count := reportAPI.arrayCount(array)
	if count < 0 || count > 100000 {
		return nil
	}
	out := make([]uintptr, 0, count)
	for i := int64(0); i < count; i++ {
		p := reportAPI.arrayGet(array, i)
		if p != 0 && reportAPI.typeID(p) == reportAPI.dictType() {
			out = append(out, p)
		}
	}
	return out
}
func reportChannel(p uintptr, readValue bool) appleEnergyChannel {
	c := appleEnergyChannel{Driver: reportAPI.driver(p), ID: reportAPI.id(p), Group: cfText(reportAPI.group(p)), Subgroup: cfText(reportAPI.subgroup(p)), Name: cfText(reportAPI.name(p)), Unit: strings.TrimSpace(cfText(reportAPI.unit(p)))}
	// Only call the simple-value accessor for recognized scalar energy channels.
	if readValue && c.cpuSource() != "" {
		c.Energy = reportAPI.integer(p, 0)
	}
	return c
}
func clpcCPUChannel(driver, id uint64) uintptr {
	channel := cfDict()
	if channel == 0 {
		return 0
	}
	legend, info := cfArray(), cfDict()
	defer cfRelease(legend)
	defer cfRelease(info)
	if legend == 0 || info == 0 {
		cfRelease(channel)
		return 0
	}
	for _, p := range []uintptr{cfNumber(int64(id)), cfNumber(1 | (2 << 16) | (1 << 32)), cfString("CPU Energy")} {
		if p == 0 {
			cfRelease(channel)
			return 0
		}
		reportAPI.arrayAppend(legend, p)
		cfRelease(p)
	}
	unit := cfNumber((3 << 56) | (118 << 32))
	cfSet(info, "IOReportChannelUnit", unit)
	cfRelease(unit)
	for _, e := range []struct {
		k string
		v uintptr
	}{
		{"DriverID", cfNumber(int64(driver))}, {"DriverName", cfString("AppleCLPC")},
		{"IOReportGroupName", cfString("CLPC")}, {"IOReportSubGroupName", cfString("Energy Counters")},
	} {
		cfSet(channel, e.k, e.v)
		cfRelease(e.v)
	}
	cfSet(channel, "IOReportChannelInfo", info)
	cfSet(channel, "LegendChannel", legend)
	return channel
}
func addCLPCChannels(ctx context.Context, selected uintptr, existing map[string]bool, version string) []string {
	var problems []string
	if strings.Split(version, ".")[0] != "27" {
		return nil
	}
	var iterator uint32
	if code := smcAPI.services(0, smcAPI.matching("AppleCLPC"), &iterator); code != 0 {
		return []string{fmt.Sprintf("AppleCLPC enumeration: 0x%x", uint32(code))}
	}
	if iterator == 0 {
		return []string{"AppleCLPC service unavailable."}
	}
	defer smcAPI.release(iterator)
	key := cfString("CFBundleIdentifier")
	defer cfRelease(key)
	for n := 0; n < 64 && ctx.Err() == nil; n++ {
		entry := smcAPI.next(iterator)
		if entry == 0 {
			break
		}
		var driver uint64
		code := reportAPI.registryID(entry, &driver)
		prop := reportAPI.property(entry, key, 0, 0)
		bundle := cfText(prop)
		cfRelease(prop)
		smcAPI.release(entry)
		if code != 0 {
			problems = append(problems, fmt.Sprintf("AppleCLPC registry ID: 0x%x", uint32(code)))
			continue
		}
		id, ok := appleCLPCCPUReportID(bundle, version)
		if !ok {
			problems = append(problems, "Unmapped AppleCLPC driver: "+bundle)
			continue
		}
		identity := fmt.Sprintf("%x:%x", driver, uint32(id))
		if existing[identity] {
			continue
		}
		existing[identity] = true
		channel := clpcCPUChannel(driver, id)
		if channel != 0 {
			reportAPI.arrayAppend(selected, channel)
			cfRelease(channel)
		}
	}
	return problems
}
func readIOReportCPUPower(ctx context.Context) CPUPowerReport {
	r := CPUPowerReport{At: time.Now()}
	fail := func(err string) CPUPowerReport { r.Problems = append(r.Problems, err); return r }
	if err := ctx.Err(); err != nil {
		return fail(err.Error())
	}
	reportAPI.once.Do(initReportAPI)
	if reportAPI.err != nil {
		return fail("IOReport initialization: " + reportAPI.err.Error())
	}
	version, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return fail("macOS product version unavailable: " + err.Error())
	}
	all := reportAPI.all(0, 0)
	defer cfRelease(all)
	var requested uintptr
	if all != 0 {
		requested = reportAPI.dictCopy(0, 0, all)
	} else {
		requested = cfDict()
	}
	if requested == 0 {
		return fail("IOReport channel dictionary unavailable")
	}
	defer cfRelease(requested)
	selected := cfArray()
	if selected == 0 {
		return fail("IOReport channel array unavailable")
	}
	defer cfRelease(selected)
	existing := map[string]bool{}
	for _, p := range reportChannels(all) {
		c := reportChannel(p, false)
		if c.cpuSource() == "" {
			continue
		}
		// macOS 27 may expose frozen legacy counters. Never subscribe to those.
		if appleLegacyEnergyBlocked(version) && c.cpuSource() != "CLPC" {
			continue
		}
		identity := c.identity()
		if c.cpuSource() == "CLPC" {
			identity = fmt.Sprintf("%x:%x", c.Driver, uint32(c.ID))
		}
		if existing[identity] {
			continue
		}
		existing[identity] = true
		reportAPI.arrayAppend(selected, p)
	}
	r.Problems = append(r.Problems, addCLPCChannels(ctx, selected, existing, version)...)
	cfSet(requested, "IOReportChannels", selected)
	if reportAPI.arrayCount(selected) == 0 {
		return fail("No recognized CPU energy channels on macOS " + version + ". Legacy Energy Model counters are excluded on macOS 27.")
	}
	var subscribed uintptr
	subscription := reportAPI.subscribe(0, requested, &subscribed, 0, 0)
	defer cfRelease(subscription)
	defer cfRelease(subscribed)
	if subscription == 0 || subscribed == 0 {
		return fail("IOReport CPU energy subscription failed")
	}
	var expected []appleEnergyChannel
	for _, p := range reportChannels(subscribed) {
		c := reportChannel(p, false)
		if c.cpuSource() != "" {
			expected = append(expected, c)
		}
	}
	if len(expected) == 0 {
		return fail("IOReport subscription omitted all CPU energy channels")
	}
	first := reportAPI.sample(subscription, subscribed, 0)
	defer cfRelease(first)
	if first == 0 {
		return fail("IOReport first energy sample failed")
	}
	started := time.Now()
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fail(ctx.Err().Error())
	case <-timer.C:
	}
	second := reportAPI.sample(subscription, subscribed, 0)
	defer cfRelease(second)
	elapsed := time.Since(started)
	if second == 0 {
		return fail("IOReport second energy sample failed")
	}
	delta := reportAPI.delta(first, second, 0)
	defer cfRelease(delta)
	if delta == 0 {
		return fail("IOReport CPU energy delta failed")
	}
	var values []appleEnergyChannel
	for _, p := range reportChannels(delta) {
		values = append(values, reportChannel(p, true))
	}
	parsed := appleEnergyReport(expected, values, version, elapsed)
	parsed.Problems = append(parsed.Problems, r.Problems...)
	return parsed
}
