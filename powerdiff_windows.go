//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

const (
	ERROR_MORE_DATA     = 234
	ERROR_NO_MORE_ITEMS = 259

	// POWER_DATA_ACCESSOR values from powrprof.h.
	ACCESS_SUBGROUP           = 17
	ACCESS_INDIVIDUAL_SETTING = 18
)

var (
	powrprof = syscall.NewLazyDLL("powrprof.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procPowerGetActiveScheme          = powrprof.NewProc("PowerGetActiveScheme")
	procPowerEnumerate                = powrprof.NewProc("PowerEnumerate")
	procPowerReadFriendlyName         = powrprof.NewProc("PowerReadFriendlyName")
	procPowerReadPossibleFriendlyName = powrprof.NewProc("PowerReadPossibleFriendlyName")
	procPowerReadACValueIndex         = powrprof.NewProc("PowerReadACValueIndex")
	procPowerReadDCValueIndex         = powrprof.NewProc("PowerReadDCValueIndex")
	procPowerReadACDefaultIndex       = powrprof.NewProc("PowerReadACDefaultIndex")
	procPowerReadDCDefaultIndex       = powrprof.NewProc("PowerReadDCDefaultIndex")
	procLocalFree                     = kernel32.NewProc("LocalFree")

	noSubgroupGUID = mustParseGUID("fea3413e-7e05-4911-9a71-700331f1c294")
)

func main() {
	schemeArg := flag.String("scheme", "active", "电源方案 GUID；默认 active 表示当前活动电源方案")
	showAll := flag.Bool("all", false, "同时显示读取失败的项目，便于排查")
	flag.Parse()

	var scheme GUID
	var err error

	if strings.EqualFold(strings.TrimSpace(*schemeArg), "active") || strings.TrimSpace(*schemeArg) == "" {
		scheme, err = getActiveScheme()
	} else {
		scheme, err = parseGUID(*schemeArg)
	}
	if err != nil {
		fatalf("获取电源方案失败: %v", err)
	}

	schemeName := readFriendlyName(&scheme, nil, nil)
	if schemeName == "" {
		schemeName = scheme.String()
	}

	fmt.Printf("Power scheme: %s\n", schemeName)
	fmt.Printf("Scheme GUID : %s\n\n", scheme.String())

	subgroups := []GUID{noSubgroupGUID}
	moreSubgroups, err := enumerateGUIDs(&scheme, nil, ACCESS_SUBGROUP)
	if err != nil {
		fatalf("枚举 subgroup 失败: %v", err)
	}
	subgroups = append(subgroups, moreSubgroups...)

	diffCount := 0

	for _, subgroup := range subgroups {
		settings, err := enumerateGUIDs(&scheme, &subgroup, ACCESS_INDIVIDUAL_SETTING)
		if err != nil {
			if *showAll {
				fmt.Fprintf(os.Stderr, "跳过 subgroup %s: %v\n", subgroup.String(), err)
			}
			continue
		}

		subgroupName := "none"
		if subgroup != noSubgroupGUID {
			subgroupName = readFriendlyName(&scheme, &subgroup, nil)
			if subgroupName == "" {
				subgroupName = subgroup.String()
			}
		}

		for _, setting := range settings {
			settingName := readFriendlyName(&scheme, &subgroup, &setting)
			if settingName == "" {
				settingName = setting.String()
			}

			var diffs []string
			var readErrors []string

			acValue, acValueErr := readIndex(procPowerReadACValueIndex, &scheme, &subgroup, &setting)
			acDefault, acDefaultErr := readIndex(procPowerReadACDefaultIndex, &scheme, &subgroup, &setting)
			if acValueErr == nil && acDefaultErr == nil {
				if acValue != acDefault {
					diffs = append(diffs, fmt.Sprintf(
						"AC: current=%s, default=%s",
						formatValue(&subgroup, &setting, acValue),
						formatValue(&subgroup, &setting, acDefault),
					))
				}
			} else if *showAll {
				readErrors = append(readErrors, fmt.Sprintf("AC read error: current=%v, default=%v", acValueErr, acDefaultErr))
			}

			dcValue, dcValueErr := readIndex(procPowerReadDCValueIndex, &scheme, &subgroup, &setting)
			dcDefault, dcDefaultErr := readIndex(procPowerReadDCDefaultIndex, &scheme, &subgroup, &setting)
			if dcValueErr == nil && dcDefaultErr == nil {
				if dcValue != dcDefault {
					diffs = append(diffs, fmt.Sprintf(
						"DC: current=%s, default=%s",
						formatValue(&subgroup, &setting, dcValue),
						formatValue(&subgroup, &setting, dcDefault),
					))
				}
			} else if *showAll {
				readErrors = append(readErrors, fmt.Sprintf("DC read error: current=%v, default=%v", dcValueErr, dcDefaultErr))
			}

			if len(diffs) == 0 && len(readErrors) == 0 {
				continue
			}

			if len(diffs) > 0 {
				diffCount++
				fmt.Printf("[%d] %s\n", diffCount, settingName)
			} else {
				fmt.Printf("[-] %s\n", settingName)
			}
			fmt.Printf("    Subgroup     : %s\n", subgroupName)
			fmt.Printf("    Subgroup GUID: %s\n", subgroup.String())
			fmt.Printf("    Setting GUID : %s\n", setting.String())

			for _, diff := range diffs {
				fmt.Printf("    %s\n", diff)
			}
			for _, readErr := range readErrors {
				fmt.Printf("    %s\n", readErr)
			}
			fmt.Println()
		}
	}

	if diffCount == 0 {
		fmt.Println("没有发现与默认值不同的设置。")
	}
}

func getActiveScheme() (GUID, error) {
	var schemePtr uintptr
	ret, _, _ := procPowerGetActiveScheme.Call(
		0,
		uintptr(unsafe.Pointer(&schemePtr)),
	)
	if ret != 0 {
		return GUID{}, syscall.Errno(ret)
	}
	if schemePtr == 0 {
		return GUID{}, fmt.Errorf("PowerGetActiveScheme 返回空指针")
	}
	defer procLocalFree.Call(schemePtr)

	return *(*GUID)(unsafe.Pointer(schemePtr)), nil
}

func enumerateGUIDs(scheme *GUID, subgroup *GUID, access uint32) ([]GUID, error) {
	var result []GUID
	for index := uint32(0); ; index++ {
		var guid GUID
		bufferSize := uint32(unsafe.Sizeof(guid))

		ret, _, _ := procPowerEnumerate.Call(
			0,
			guidPtr(scheme),
			guidPtr(subgroup),
			uintptr(access),
			uintptr(index),
			uintptr(unsafe.Pointer(&guid)),
			uintptr(unsafe.Pointer(&bufferSize)),
		)

		if ret == 0 {
			result = append(result, guid)
			continue
		}
		if ret == ERROR_NO_MORE_ITEMS {
			return result, nil
		}
		if index == 0 {
			return nil, syscall.Errno(ret)
		}
		return result, nil
	}
}

func readIndex(proc *syscall.LazyProc, scheme *GUID, subgroup *GUID, setting *GUID) (uint32, error) {
	var value uint32
	ret, _, _ := proc.Call(
		0,
		guidPtr(scheme),
		guidPtr(subgroup),
		guidPtr(setting),
		uintptr(unsafe.Pointer(&value)),
	)
	if ret != 0 {
		return 0, syscall.Errno(ret)
	}
	return value, nil
}

func readFriendlyName(scheme *GUID, subgroup *GUID, setting *GUID) string {
	buf := make([]uint16, 2048)
	bufferSize := uint32(len(buf) * 2)

	ret, _, _ := procPowerReadFriendlyName.Call(
		0,
		guidPtr(scheme),
		guidPtr(subgroup),
		guidPtr(setting),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufferSize)),
	)
	if ret == ERROR_MORE_DATA && bufferSize > 0 {
		buf = make([]uint16, (bufferSize+1)/2)
		ret, _, _ = procPowerReadFriendlyName.Call(
			0,
			guidPtr(scheme),
			guidPtr(subgroup),
			guidPtr(setting),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&bufferSize)),
		)
	}
	if ret != 0 {
		return ""
	}
	return strings.TrimRight(syscall.UTF16ToString(buf), "\x00")
}

func formatValue(subgroup *GUID, setting *GUID, index uint32) string {
	name := readPossibleFriendlyName(subgroup, setting, index)
	if name == "" {
		return fmt.Sprintf("%d", index)
	}
	return fmt.Sprintf("%s (%d)", name, index)
}

func readPossibleFriendlyName(subgroup *GUID, setting *GUID, index uint32) string {
	buf := make([]uint16, 2048)
	bufferSize := uint32(len(buf) * 2)

	ret, _, _ := procPowerReadPossibleFriendlyName.Call(
		0,
		guidPtr(subgroup),
		guidPtr(setting),
		uintptr(index),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufferSize)),
	)
	if ret == ERROR_MORE_DATA && bufferSize > 0 {
		buf = make([]uint16, (bufferSize+1)/2)
		ret, _, _ = procPowerReadPossibleFriendlyName.Call(
			0,
			guidPtr(subgroup),
			guidPtr(setting),
			uintptr(index),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&bufferSize)),
		)
	}
	if ret != 0 {
		return ""
	}
	return strings.TrimRight(syscall.UTF16ToString(buf), "\x00")
}

func guidPtr(g *GUID) uintptr {
	if g == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(g))
}

func parseGUID(s string) (GUID, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")

	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return GUID{}, fmt.Errorf("GUID 格式错误: %q", s)
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return GUID{}, fmt.Errorf("GUID 长度错误: %q", s)
	}

	data1, err := parseHexUint32(parts[0], 32)
	if err != nil {
		return GUID{}, err
	}
	data2, err := parseHexUint32(parts[1], 16)
	if err != nil {
		return GUID{}, err
	}
	data3, err := parseHexUint32(parts[2], 16)
	if err != nil {
		return GUID{}, err
	}

	var data4 [8]byte
	for i := 0; i < 2; i++ {
		b, err := parseHexByte(parts[3][i*2 : i*2+2])
		if err != nil {
			return GUID{}, err
		}
		data4[i] = b
	}
	for i := 0; i < 6; i++ {
		b, err := parseHexByte(parts[4][i*2 : i*2+2])
		if err != nil {
			return GUID{}, err
		}
		data4[i+2] = b
	}

	return GUID{
		Data1: data1,
		Data2: uint16(data2),
		Data3: uint16(data3),
		Data4: data4,
	}, nil
}

func mustParseGUID(s string) GUID {
	guid, err := parseGUID(s)
	if err != nil {
		panic(err)
	}
	return guid
}

func parseHexUint32(s string, bitSize int) (uint32, error) {
	v, err := strconv.ParseUint(s, 16, bitSize)
	if err != nil {
		return 0, fmt.Errorf("解析十六进制失败 %q: %w", s, err)
	}
	return uint32(v), nil
}

func parseHexByte(s string) (byte, error) {
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0, fmt.Errorf("解析十六进制字节失败 %q: %w", s, err)
	}
	return byte(v), nil
}

func (g GUID) String() string {
	return fmt.Sprintf(
		"%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g.Data1,
		g.Data2,
		g.Data3,
		g.Data4[0], g.Data4[1],
		g.Data4[2], g.Data4[3], g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7],
	)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
