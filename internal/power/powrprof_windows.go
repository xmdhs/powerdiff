//go:build windows

package power

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

const (
	errorMoreData    = 234
	errorNoMoreItems = 259

	accessACPowerSettingIndex              = 0
	accessDCPowerSettingIndex              = 1
	accessFriendlyName                     = 2
	accessDescription                      = 3
	accessPossiblePowerSetting             = 4
	accessPossiblePowerSettingFriendlyName = 5
	accessPossiblePowerSettingDescription  = 6
	accessDefaultACPowerSetting            = 7
	accessDefaultDCPowerSetting            = 8
	accessPossibleValueMin                 = 9
	accessPossibleValueMax                 = 10
	accessPossibleValueIncrement           = 11
	accessPossibleValueUnits               = 12
	accessAttributes                       = 15
	accessScheme                           = 16
	accessSubgroup                         = 17
	accessIndividualSetting                = 18
	accessOverlayScheme                    = 24
	accessActiveOverlayScheme              = 25
)

var (
	powrprof = syscall.NewLazyDLL("powrprof.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procPowerGetActiveScheme          = powrprof.NewProc("PowerGetActiveScheme")
	procPowerSetActiveScheme          = powrprof.NewProc("PowerSetActiveScheme")
	procPowerEnumerate                = powrprof.NewProc("PowerEnumerate")
	procPowerReadFriendlyName         = powrprof.NewProc("PowerReadFriendlyName")
	procPowerReadDescription          = powrprof.NewProc("PowerReadDescription")
	procPowerReadPossibleFriendlyName = powrprof.NewProc("PowerReadPossibleFriendlyName")
	procPowerReadPossibleDescription  = powrprof.NewProc("PowerReadPossibleDescription")
	procPowerReadPossibleValue        = powrprof.NewProc("PowerReadPossibleValue")
	procPowerWritePossibleValue       = powrprof.NewProc("PowerWritePossibleValue")
	procPowerCreatePossibleSetting    = powrprof.NewProc("PowerCreatePossibleSetting")
	procPowerWritePossibleFriendly    = powrprof.NewProc("PowerWritePossibleFriendlyName")
	procPowerReadACValueIndex         = powrprof.NewProc("PowerReadACValueIndex")
	procPowerReadDCValueIndex         = powrprof.NewProc("PowerReadDCValueIndex")
	procPowerWriteACValueIndex        = powrprof.NewProc("PowerWriteACValueIndex")
	procPowerWriteDCValueIndex        = powrprof.NewProc("PowerWriteDCValueIndex")
	procPowerReadACDefaultIndex       = powrprof.NewProc("PowerReadACDefaultIndex")
	procPowerReadDCDefaultIndex       = powrprof.NewProc("PowerReadDCDefaultIndex")
	procPowerReadValueMin             = powrprof.NewProc("PowerReadValueMin")
	procPowerReadValueMax             = powrprof.NewProc("PowerReadValueMax")
	procPowerReadValueIncrement       = powrprof.NewProc("PowerReadValueIncrement")
	procPowerReadValueUnits           = powrprof.NewProc("PowerReadValueUnitsSpecifier")
	procPowerReadSettingAttributes    = powrprof.NewProc("PowerReadSettingAttributes")
	procPowerWriteSettingAttributes   = powrprof.NewProc("PowerWriteSettingAttributes")
	procPowerDeterminePlatformRole    = powrprof.NewProc("PowerDeterminePlatformRole")
	procPowerSetActiveOverlayScheme   = powrprof.NewProc("PowerSetActiveOverlayScheme")
	procPowerGetActualOverlayScheme   = powrprof.NewProc("PowerGetActualOverlayScheme")
	procLocalFree                     = kernel32.NewProc("LocalFree")
)

func guidPtr(g *GUID) uintptr {
	if g == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(g))
}

func winErr(ret uintptr) error {
	if ret == 0 {
		return nil
	}
	return syscall.Errno(ret)
}

func resolveScheme(s string) (GUID, error) {
	if strings.TrimSpace(s) == "" || strings.EqualFold(strings.TrimSpace(s), "active") || strings.EqualFold(strings.TrimSpace(s), "current") {
		return activeSchemeGUID()
	}
	return ParseGUID(s)
}

func Platform() PlatformInfo {
	role := platformRole()
	return PlatformInfo{Windows: true, Role: roleName(role), HasDC: hasDC(role)}
}

func platformRole() uint32 {
	ret, _, _ := procPowerDeterminePlatformRole.Call()
	return uint32(ret)
}

func roleName(role uint32) string {
	switch role {
	case 1:
		return "desktop"
	case 2:
		return "mobile"
	case 3:
		return "workstation"
	case 4:
		return "enterprise-server"
	case 5:
		return "soho-server"
	case 6:
		return "appliance-pc"
	case 7:
		return "performance-server"
	case 8:
		return "slate"
	default:
		return "unspecified"
	}
}

func hasDC(role uint32) bool { return role == 2 || role == 8 }

func activeSchemeGUID() (GUID, error) {
	var schemePtr uintptr
	ret, _, _ := procPowerGetActiveScheme.Call(0, uintptr(unsafe.Pointer(&schemePtr)))
	if ret != 0 {
		return GUID{}, winErr(ret)
	}
	if schemePtr == 0 {
		return GUID{}, fmt.Errorf("PowerGetActiveScheme 返回空指针")
	}
	defer procLocalFree.Call(schemePtr)
	return *(*GUID)(unsafe.Pointer(schemePtr)), nil
}

func ActiveScheme() (Scheme, error) {
	active, err := activeSchemeGUID()
	if err != nil {
		return Scheme{}, err
	}
	return schemeFromGUID(active, "scheme", active), nil
}

func ListSchemes() ([]Scheme, error) {
	active, _ := activeSchemeGUID()
	guids, err := enumerateGUIDs(nil, nil, accessScheme)
	if err != nil {
		return nil, err
	}
	result := make([]Scheme, 0, len(guids))
	for _, guid := range guids {
		result = append(result, schemeFromGUID(guid, "scheme", active))
	}
	return result, nil
}

func SetActiveScheme(s string) error {
	guid, err := ParseGUID(s)
	if err != nil {
		return err
	}
	ret, _, _ := procPowerSetActiveScheme.Call(0, guidPtr(&guid))
	return winErr(ret)
}

func reapplyActiveScheme() {
	guid, err := activeSchemeGUID()
	if err == nil {
		_, _, _ = procPowerSetActiveScheme.Call(0, guidPtr(&guid))
	}
}

func schemeFromGUID(guid GUID, typ string, active GUID) Scheme {
	name := readFriendlyName(&guid, nil, nil)
	if name == "" {
		name = guid.String()
	}
	return Scheme{GUID: guid.String(), Name: name, Description: readDescription(&guid, nil, nil), Active: guid == active, Type: typ}
}

func ListOverlays() ([]Scheme, error) {
	if err := procPowerSetActiveOverlayScheme.Find(); err != nil {
		return []Scheme{}, nil
	}
	guids, err := enumerateGUIDs(nil, nil, accessOverlayScheme)
	if err != nil {
		return []Scheme{}, nil
	}
	var active GUID
	if procPowerGetActualOverlayScheme.Find() == nil {
		_, _, _ = procPowerGetActualOverlayScheme.Call(uintptr(unsafe.Pointer(&active)))
	}
	result := make([]Scheme, 0, len(guids))
	for _, guid := range guids {
		result = append(result, schemeFromGUID(guid, "overlay", active))
	}
	return result, nil
}

func SetActiveOverlay(s string) error {
	if err := procPowerSetActiveOverlayScheme.Find(); err != nil {
		return fmt.Errorf("当前系统不支持 power overlay: %w", err)
	}
	guid, err := ParseGUID(s)
	if err != nil {
		return err
	}
	ret, _, _ := procPowerSetActiveOverlayScheme.Call(guidPtr(&guid))
	return winErr(ret)
}

func enumerateGUIDs(scheme *GUID, subgroup *GUID, access uint32) ([]GUID, error) {
	var result []GUID
	for index := uint32(0); ; index++ {
		var guid GUID
		bufferSize := uint32(unsafe.Sizeof(guid))
		ret, _, _ := procPowerEnumerate.Call(0, guidPtr(scheme), guidPtr(subgroup), uintptr(access), uintptr(index), uintptr(unsafe.Pointer(&guid)), uintptr(unsafe.Pointer(&bufferSize)))
		if ret == 0 {
			result = append(result, guid)
			continue
		}
		if ret == errorNoMoreItems {
			return result, nil
		}
		if index == 0 {
			return nil, winErr(ret)
		}
		return result, nil
	}
}

func readString(proc *syscall.LazyProc, scheme, subgroup, setting *GUID) string {
	buf := make([]uint16, 2048)
	bufferSize := uint32(len(buf) * 2)
	ret, _, _ := proc.Call(0, guidPtr(scheme), guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufferSize)))
	if ret == errorMoreData && bufferSize > 0 {
		buf = make([]uint16, (bufferSize+1)/2)
		ret, _, _ = proc.Call(0, guidPtr(scheme), guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufferSize)))
	}
	if ret != 0 {
		return ""
	}
	return strings.TrimRight(syscall.UTF16ToString(buf), "\x00")
}

func readFriendlyName(scheme, subgroup, setting *GUID) string {
	return readString(procPowerReadFriendlyName, scheme, subgroup, setting)
}

func readDescription(scheme, subgroup, setting *GUID) string {
	return readString(procPowerReadDescription, scheme, subgroup, setting)
}

func readIndex(proc *syscall.LazyProc, scheme, subgroup, setting *GUID) (uint32, error) {
	var value uint32
	ret, _, _ := proc.Call(0, guidPtr(scheme), guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&value)))
	if ret != 0 {
		return 0, winErr(ret)
	}
	return value, nil
}

func writeIndex(proc *syscall.LazyProc, scheme, subgroup, setting *GUID, value uint32) error {
	ret, _, _ := proc.Call(0, guidPtr(scheme), guidPtr(subgroup), guidPtr(setting), uintptr(value))
	return winErr(ret)
}

func readValueMeta(proc *syscall.LazyProc, subgroup, setting *GUID) (uint32, error) {
	var value uint32
	ret, _, _ := proc.Call(0, guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&value)))
	if ret != 0 {
		return 0, winErr(ret)
	}
	return value, nil
}

func readUnits(subgroup, setting *GUID) string {
	buf := make([]uint16, 256)
	bufferSize := uint32(len(buf) * 2)
	ret, _, _ := procPowerReadValueUnits.Call(0, guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufferSize)))
	if ret == errorMoreData && bufferSize > 0 {
		buf = make([]uint16, (bufferSize+1)/2)
		ret, _, _ = procPowerReadValueUnits.Call(0, guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufferSize)))
	}
	if ret != 0 {
		return ""
	}
	return strings.TrimRight(syscall.UTF16ToString(buf), "\x00")
}

func readAttributes(subgroup, setting *GUID) uint32 {
	ret, _, _ := procPowerReadSettingAttributes.Call(guidPtr(subgroup), guidPtr(setting))
	return uint32(ret)
}

func writeAttributes(subgroup, setting *GUID, attrs uint32) error {
	ret, _, _ := procPowerWriteSettingAttributes.Call(guidPtr(subgroup), guidPtr(setting), uintptr(attrs))
	return winErr(ret)
}

func ListSettings(schemeArg string) ([]Setting, error) {
	scheme, err := resolveScheme(schemeArg)
	if err != nil {
		return nil, err
	}
	subgroups := []GUID{NoSubgroupGUID}
	more, err := enumerateGUIDs(&scheme, nil, accessSubgroup)
	if err == nil {
		subgroups = append(subgroups, more...)
	}
	result := make([]Setting, 0)
	for _, subgroup := range subgroups {
		settings, err := enumerateGUIDs(&scheme, &subgroup, accessIndividualSetting)
		if err != nil {
			continue
		}
		subgroupName := "none"
		if subgroup != NoSubgroupGUID {
			subgroupName = readFriendlyName(&scheme, &subgroup, nil)
			if subgroupName == "" {
				subgroupName = subgroup.String()
			}
		}
		for _, settingGUID := range settings {
			setting := buildSetting(scheme, subgroup, settingGUID, subgroupName)
			result = append(result, setting)
		}
	}
	return result, nil
}

func buildSetting(scheme, subgroup, settingGUID GUID, subgroupName string) Setting {
	name := readFriendlyName(&scheme, &subgroup, &settingGUID)
	if name == "" {
		name = settingGUID.String()
	}
	setting := Setting{
		GUID:           settingGUID.String(),
		SubgroupGUID:   subgroup.String(),
		Subgroup:       subgroupName,
		Name:           name,
		Description:    readDescription(&scheme, &subgroup, &settingGUID),
		Hidden:         readAttributes(&subgroup, &settingGUID)&1 != 0,
		CanAddPossible: IsHeteroThresholdGUID(settingGUID),
	}
	min, minErr := readValueMeta(procPowerReadValueMin, &subgroup, &settingGUID)
	max, maxErr := readValueMeta(procPowerReadValueMax, &subgroup, &settingGUID)
	inc, incErr := readValueMeta(procPowerReadValueIncrement, &subgroup, &settingGUID)
	if minErr == nil && maxErr == nil && incErr == nil {
		setting.IsRanged = true
		setting.Min = &min
		setting.Max = &max
		setting.Increment = &inc
		setting.Units = readUnits(&subgroup, &settingGUID)
	} else {
		setting.PossibleValues = readPossibleValues(&scheme, &subgroup, &settingGUID)
	}
	value := SettingValue{SchemeGUID: scheme.String(), HasDC: Platform().HasDC}
	if ac, err := readIndex(procPowerReadACValueIndex, &scheme, &subgroup, &settingGUID); err == nil {
		value.AC = &ac
	}
	if dc, err := readIndex(procPowerReadDCValueIndex, &scheme, &subgroup, &settingGUID); err == nil {
		value.DC = &dc
		value.HasDC = true
	}
	if acd, err := readIndex(procPowerReadACDefaultIndex, &scheme, &subgroup, &settingGUID); err == nil {
		value.ACDefault = &acd
	}
	if dcd, err := readIndex(procPowerReadDCDefaultIndex, &scheme, &subgroup, &settingGUID); err == nil {
		value.DCDefault = &dcd
		value.HasDC = true
	}
	setting.Values = []SettingValue{value}
	return setting
}

func readPossibleString(proc *syscall.LazyProc, subgroup, setting *GUID, index uint32) string {
	buf := make([]uint16, 512)
	bufferSize := uint32(len(buf) * 2)
	ret, _, _ := proc.Call(0, guidPtr(subgroup), guidPtr(setting), uintptr(index), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufferSize)))
	if ret == errorMoreData && bufferSize > 0 {
		buf = make([]uint16, (bufferSize+1)/2)
		ret, _, _ = proc.Call(0, guidPtr(subgroup), guidPtr(setting), uintptr(index), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufferSize)))
	}
	if ret != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func readPossibleValues(scheme, subgroup, setting *GUID) []PossibleValue {
	indexes := map[uint32]bool{}
	startIndex := findFirstPossibleValueIndex(scheme, subgroup, setting)
	if startIndex >= 0 {
		for i := uint32(startIndex); i < 512; i++ {
			if !possibleValueExists(subgroup, setting, i) {
				break
			}
			indexes[i] = true
		}
	}
	for _, proc := range []*syscall.LazyProc{procPowerReadACValueIndex, procPowerReadDCValueIndex, procPowerReadACDefaultIndex, procPowerReadDCDefaultIndex} {
		if v, err := readIndex(proc, scheme, subgroup, setting); err == nil {
			indexes[v] = true
		}
	}
	values := make([]PossibleValue, 0, len(indexes))
	for index := range indexes {
		if pv, err := readPossibleValue(subgroup, setting, index); err == nil {
			values = append(values, pv)
		}
	}
	return values
}

func findFirstPossibleValueIndex(scheme, subgroup, setting *GUID) int {
	if possibleValueExists(subgroup, setting, 0) {
		return 0
	}
	var acValue uint32
	ret, _, _ := procPowerReadACValueIndex.Call(0, guidPtr(scheme), guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&acValue)))
	if ret != 0 {
		return -1
	}
	num := int(acValue)
	for num > 0 {
		if possibleValueExists(subgroup, setting, uint32(num-1)) {
			num--
		} else {
			break
		}
	}
	if possibleValueExists(subgroup, setting, uint32(num)) {
		return num
	}
	return -1
}

func possibleValueExists(subgroup, setting *GUID, index uint32) bool {
	var typ RegType
	var size uint32
	ret, _, _ := procPowerReadPossibleValue.Call(0, guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&typ)), uintptr(index), 0, uintptr(unsafe.Pointer(&size)))
	return ret == 0 || ret == errorMoreData
}

func readPossibleValue(subgroup, setting *GUID, index uint32) (PossibleValue, error) {
	var typ RegType
	var size uint32
	ret, _, _ := procPowerReadPossibleValue.Call(0, guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&typ)), uintptr(index), 0, uintptr(unsafe.Pointer(&size)))
	if ret != 0 && ret != errorMoreData {
		return PossibleValue{}, winErr(ret)
	}
	if size == 0 {
		size = 4
	}
	buf := make([]byte, size)
	ret, _, _ = procPowerReadPossibleValue.Call(0, guidPtr(subgroup), guidPtr(setting), uintptr(unsafe.Pointer(&typ)), uintptr(index), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret != 0 {
		return PossibleValue{}, winErr(ret)
	}
	if uint32(len(buf)) > size {
		buf = buf[:size]
	}
	name := readPossibleString(procPowerReadPossibleFriendlyName, subgroup, setting, index)
	if name == "" {
		name = fmt.Sprintf("Value #%d", index)
	}
	if IsHeteroThresholdGUID(*setting) && len(buf) > 0 {
		parts := make([]string, len(buf))
		for i, b := range buf {
			parts[i] = strconv.Itoa(int(b))
		}
		name = strings.Join(parts, ",")
	}
	return PossibleValue{Index: index, Name: name, Description: readPossibleString(procPowerReadPossibleDescription, subgroup, setting, index), Value: decodeRegValue(typ, buf), RegType: typ.String(), RawHex: strings.ToUpper(hex.EncodeToString(buf))}, nil
}

func decodeRegValue(typ RegType, buf []byte) string {
	switch typ {
	case RegSZ, RegExpandSZ, RegLink:
		return utf16BytesToString(buf)
	case RegDWORD:
		if len(buf) >= 4 {
			return fmt.Sprintf("%08X", binary.LittleEndian.Uint32(buf))
		}
	case RegDWORDBigEndian:
		if len(buf) >= 4 {
			return fmt.Sprintf("%08X", binary.BigEndian.Uint32(buf))
		}
	case RegQWORD:
		if len(buf) >= 8 {
			return fmt.Sprintf("%016X", binary.LittleEndian.Uint64(buf))
		}
	case RegMultiSZ:
		return strings.Join(strings.FieldsFunc(utf16BytesToString(buf), func(r rune) bool { return r == 0 }), "+")
	case RegBinary:
		parts := make([]string, len(buf))
		for i, b := range buf {
			parts[i] = fmt.Sprintf("%02X", b)
		}
		return strings.Join(parts, " ")
	}
	return strings.ToUpper(hex.EncodeToString(buf))
}

func utf16BytesToString(buf []byte) string {
	if len(buf) < 2 {
		return ""
	}
	words := make([]uint16, 0, len(buf)/2)
	for i := 0; i+1 < len(buf); i += 2 {
		words = append(words, binary.LittleEndian.Uint16(buf[i:i+2]))
	}
	return strings.TrimRight(syscall.UTF16ToString(words), "\x00")
}

func UpdateSetting(settingArg string, req UpdateSettingRequest) (SettingValue, error) {
	scheme, err := resolveScheme(req.Scheme)
	if err != nil {
		return SettingValue{}, err
	}
	subgroup, err := ParseGUID(req.Subgroup)
	if err != nil {
		return SettingValue{}, err
	}
	setting, err := ParseGUID(settingArg)
	if err != nil {
		return SettingValue{}, err
	}
	meta := buildSetting(scheme, subgroup, setting, "")
	if req.AC != nil {
		if err := validateValue(meta, *req.AC); err != nil {
			return SettingValue{}, err
		}
		if err := writeIndex(procPowerWriteACValueIndex, &scheme, &subgroup, &setting, *req.AC); err != nil {
			return SettingValue{}, err
		}
	}
	if req.DC != nil {
		if err := validateValue(meta, *req.DC); err != nil {
			return SettingValue{}, err
		}
		if err := writeIndex(procPowerWriteDCValueIndex, &scheme, &subgroup, &setting, *req.DC); err != nil {
			return SettingValue{}, err
		}
	}
	if active, err := activeSchemeGUID(); err == nil && active == scheme {
		reapplyActiveScheme()
	}
	value := SettingValue{SchemeGUID: scheme.String(), HasDC: Platform().HasDC}
	if ac, err := readIndex(procPowerReadACValueIndex, &scheme, &subgroup, &setting); err == nil {
		value.AC = &ac
	}
	if dc, err := readIndex(procPowerReadDCValueIndex, &scheme, &subgroup, &setting); err == nil {
		value.DC = &dc
		value.HasDC = true
	}
	return value, nil
}

func validateValue(setting Setting, value uint32) error {
	if setting.IsRanged {
		if setting.Min != nil && value < *setting.Min {
			return fmt.Errorf("值 %d 小于最小值 %d", value, *setting.Min)
		}
		if setting.Max != nil && value > *setting.Max {
			return fmt.Errorf("值 %d 大于最大值 %d", value, *setting.Max)
		}
		if setting.Increment != nil && *setting.Increment > 1 && setting.Min != nil && (value-*setting.Min)%*setting.Increment != 0 {
			return fmt.Errorf("值 %d 不符合步进 %d", value, *setting.Increment)
		}
		return nil
	}
	for _, pv := range setting.PossibleValues {
		if pv.Index == value {
			return nil
		}
	}
	return fmt.Errorf("值 %d 不在 possible values 中", value)
}

func SetHidden(settingArg string, req HiddenRequest) error {
	subgroup, err := ParseGUID(req.Subgroup)
	if err != nil {
		return err
	}
	setting, err := ParseGUID(settingArg)
	if err != nil {
		return err
	}
	attrs := readAttributes(&subgroup, &setting)
	if req.Hidden {
		attrs |= 1
	} else {
		attrs &^= 1
		attrs |= 2
	}
	if err := writeAttributes(&subgroup, &setting, attrs); err != nil {
		return err
	}
	if !req.Hidden && subgroup != NoSubgroupGUID {
		subAttrs := readAttributes(&subgroup, nil)
		if subAttrs == 1 {
			regPath := `SYSTEM\CurrentControlSet\Control\Power\PowerSettings\` + subgroup.String()
			if k, err := registry.OpenKey(registry.LOCAL_MACHINE, regPath, registry.SET_VALUE); err == nil {
				if delErr := k.DeleteValue("Attributes"); delErr != nil {
					_ = writeAttributes(&subgroup, nil, 0)
				}
				_ = k.Close()
			} else {
				_ = writeAttributes(&subgroup, nil, 0)
			}
		} else if subAttrs&1 != 0 {
			subAttrs &^= 1
			_ = writeAttributes(&subgroup, nil, subAttrs)
		}
	}
	reapplyActiveScheme()
	return nil
}

func AddPossibleValue(settingArg string, req PossibleValueRequest) error {
	subgroup, err := ParseGUID(req.Subgroup)
	if err != nil {
		return err
	}
	setting, err := ParseGUID(settingArg)
	if err != nil {
		return err
	}
	if !IsHeteroThresholdGUID(setting) {
		return fmt.Errorf("该设置不支持添加 possible value")
	}
	raw, err := hex.DecodeString(strings.ReplaceAll(req.RawHex, " ", ""))
	if err != nil {
		return err
	}
	ret, _, _ := procPowerCreatePossibleSetting.Call(0, guidPtr(&subgroup), guidPtr(&setting), uintptr(req.Index))
	if ret != 0 {
		return winErr(ret)
	}
	name := fmt.Sprintf("(%d) %s", req.Index, strings.ToUpper(hex.EncodeToString(raw)))
	name16, _ := syscall.UTF16FromString(name)
	_, _, _ = procPowerWritePossibleFriendly.Call(0, guidPtr(&subgroup), guidPtr(&setting), uintptr(req.Index), uintptr(unsafe.Pointer(&name16[0])), uintptr(len(name16)*2))
	typ := RegBinary
	ret, _, _ = procPowerWritePossibleValue.Call(0, guidPtr(&subgroup), guidPtr(&setting), uintptr(typ), uintptr(req.Index), uintptr(unsafe.Pointer(&raw[0])), uintptr(len(raw)))
	return winErr(ret)
}

func DiffScheme(schemeArg string, showReadErrors bool) ([]DiffItem, error) {
	settings, err := ListSettings(schemeArg)
	if err != nil {
		return nil, err
	}
	items := make([]DiffItem, 0)
	for _, setting := range settings {
		if len(setting.Values) == 0 {
			continue
		}
		value := setting.Values[0]
		item := DiffItem{SettingGUID: setting.GUID, SubgroupGUID: setting.SubgroupGUID, Setting: setting.Name, Subgroup: setting.Subgroup, Description: setting.Description}
		if value.AC != nil && value.ACDefault != nil && *value.AC != *value.ACDefault {
			item.Diffs = append(item.Diffs, ValueDiff{Source: "AC", Current: *value.AC, Default: *value.ACDefault, CurrentText: formatSettingValue(setting, *value.AC), DefaultText: formatSettingValue(setting, *value.ACDefault)})
		}
		if value.DC != nil && value.DCDefault != nil && *value.DC != *value.DCDefault {
			item.Diffs = append(item.Diffs, ValueDiff{Source: "DC", Current: *value.DC, Default: *value.DCDefault, CurrentText: formatSettingValue(setting, *value.DC), DefaultText: formatSettingValue(setting, *value.DCDefault)})
		}
		if len(item.Diffs) > 0 || (showReadErrors && len(item.ReadErrors) > 0) {
			items = append(items, item)
		}
	}
	return items, nil
}

func DiffAgainst(schemeA, schemeB string) ([]DiffItem, error) {
	settingsA, err := ListSettings(schemeA)
	if err != nil {
		return nil, err
	}
	settingsB, err := ListSettings(schemeB)
	if err != nil {
		return nil, err
	}
	bMap := make(map[string]Setting)
	for _, s := range settingsB {
		bMap[s.GUID] = s
	}
	items := make([]DiffItem, 0)
	for _, sa := range settingsA {
		sb, ok := bMap[sa.GUID]
		if !ok || len(sa.Values) == 0 || len(sb.Values) == 0 {
			continue
		}
		va := sa.Values[0]
		vb := sb.Values[0]
		item := DiffItem{SettingGUID: sa.GUID, SubgroupGUID: sa.SubgroupGUID, Setting: sa.Name, Subgroup: sa.Subgroup, Description: sa.Description}
		if va.AC != nil && vb.AC != nil && *va.AC != *vb.AC {
			item.Diffs = append(item.Diffs, ValueDiff{Source: "AC", Current: *va.AC, Default: *vb.AC, CurrentText: formatSettingValue(sa, *va.AC), DefaultText: formatSettingValue(sa, *vb.AC)})
		}
		if va.DC != nil && vb.DC != nil && *va.DC != *vb.DC {
			item.Diffs = append(item.Diffs, ValueDiff{Source: "DC", Current: *va.DC, Default: *vb.DC, CurrentText: formatSettingValue(sa, *va.DC), DefaultText: formatSettingValue(sa, *vb.DC)})
		}
		if len(item.Diffs) > 0 {
			items = append(items, item)
		}
	}
	return items, nil
}

func formatSettingValue(setting Setting, value uint32) string {
	if setting.IsRanged {
		if setting.Units != "" {
			return fmt.Sprintf("%d %s", value, setting.Units)
		}
		return fmt.Sprintf("%d", value)
	}
	for _, pv := range setting.PossibleValues {
		if pv.Index == value {
			return fmt.Sprintf("%s (%d)", pv.Name, value)
		}
	}
	return fmt.Sprintf("%d", value)
}

type xmlRoot struct {
	XMLName   xml.Name      `xml:"root"`
	Subgroups []xmlSubgroup `xml:"subgroup"`
}

type xmlSubgroup struct {
	GUID     string       `xml:"guid,attr"`
	Settings []xmlSetting `xml:"setting"`
}

type xmlSetting struct {
	GUID    string     `xml:"guid,attr"`
	Name    string     `xml:"name,attr,omitempty"`
	ACIndex []xmlIndex `xml:"acindex"`
	DCIndex []xmlIndex `xml:"dcindex"`
}

type xmlIndex struct {
	Scheme string `xml:"scheme,attr"`
	Value  uint32 `xml:"value,attr"`
}

func ExportXML(schemeArg string) ([]byte, error) {
	schemes, err := ListSchemes()
	if err != nil {
		return nil, err
	}
	overlays, _ := ListOverlays()
	schemes = append(schemes, overlays...)
	baseSettings, err := ListSettings(schemeArg)
	if err != nil {
		return nil, err
	}
	root := xmlRoot{}
	bySubgroup := map[string]int{}
	for _, setting := range baseSettings {
		idx, ok := bySubgroup[setting.SubgroupGUID]
		if !ok {
			root.Subgroups = append(root.Subgroups, xmlSubgroup{GUID: setting.SubgroupGUID})
			idx = len(root.Subgroups) - 1
			bySubgroup[setting.SubgroupGUID] = idx
		}
		xs := xmlSetting{GUID: setting.GUID, Name: setting.Name}
		subgroup, _ := ParseGUID(setting.SubgroupGUID)
		settingGUID, _ := ParseGUID(setting.GUID)
		for _, scheme := range schemes {
			sg, err := ParseGUID(scheme.GUID)
			if err != nil {
				continue
			}
			if ac, err := readIndex(procPowerReadACValueIndex, &sg, &subgroup, &settingGUID); err == nil {
				xs.ACIndex = append(xs.ACIndex, xmlIndex{Scheme: sg.String(), Value: ac})
			}
			if dc, err := readIndex(procPowerReadDCValueIndex, &sg, &subgroup, &settingGUID); err == nil {
				xs.DCIndex = append(xs.DCIndex, xmlIndex{Scheme: sg.String(), Value: dc})
			}
		}
		root.Subgroups[idx].Settings = append(root.Subgroups[idx].Settings, xs)
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ImportXML(data []byte) (ImportResult, error) {
	var root xmlRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return ImportResult{}, err
	}
	result := ImportResult{}
	for _, subgroupXML := range root.Subgroups {
		subgroup, err := ParseGUID(subgroupXML.GUID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		for _, settingXML := range subgroupXML.Settings {
			setting, err := ParseGUID(settingXML.GUID)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			for _, item := range settingXML.ACIndex {
				scheme, err := ParseGUID(item.Scheme)
				if err == nil {
					err = writeIndex(procPowerWriteACValueIndex, &scheme, &subgroup, &setting, item.Value)
				}
				if err != nil {
					result.Failed++
					result.Errors = append(result.Errors, err.Error())
				} else {
					result.Applied++
				}
			}
			for _, item := range settingXML.DCIndex {
				scheme, err := ParseGUID(item.Scheme)
				if err == nil {
					err = writeIndex(procPowerWriteDCValueIndex, &scheme, &subgroup, &setting, item.Value)
				}
				if err != nil {
					result.Failed++
					result.Errors = append(result.Errors, err.Error())
				} else {
					result.Applied++
				}
			}
		}
	}
	reapplyActiveScheme()
	return result, nil
}

func ExportScript(schemeArg string) (string, error) {
	settings, err := ListSettings(schemeArg)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, setting := range settings {
		if len(setting.Values) == 0 {
			continue
		}
		v := setting.Values[0]
		b.WriteString("rem " + setting.Subgroup + "\\" + setting.Name + "\r\n")
		if v.AC != nil {
			fmt.Fprintf(&b, "powercfg /setacvalueindex %s %s %s %d\r\n", v.SchemeGUID, setting.SubgroupGUID, setting.GUID, *v.AC)
		}
		if v.DC != nil {
			fmt.Fprintf(&b, "powercfg /setdcvalueindex %s %s %s %d\r\n", v.SchemeGUID, setting.SubgroupGUID, setting.GUID, *v.DC)
		}
	}
	b.WriteString("rem To apply changes\r\npowercfg /setactive SCHEME_CURRENT\r\n")
	return b.String(), nil
}
