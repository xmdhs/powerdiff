package power

import (
	"fmt"
	"strconv"
	"strings"
)

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	NoSubgroupGUID                = MustParseGUID("fea3413e-7e05-4911-9a71-700331f1c294")
	PowerPlanTypeGUID             = MustParseGUID("245d8541-3943-4422-b025-13a784f679b7")
	HeteroIncreaseThresholdClass1 = MustParseGUID("b000397d-9b0b-483d-98c9-692a6060cfbf")
	HeteroDecreaseThresholdClass1 = MustParseGUID("f8861c27-95e7-475c-865b-13c0cb3f9d6b")
	HeteroIncreaseThresholdClass2 = MustParseGUID("b000397d-9b0b-483d-98c9-692a6060cfc0")
	HeteroDecreaseThresholdClass2 = MustParseGUID("f8861c27-95e7-475c-865b-13c0cb3f9d6c")
	CurrentSchemeGUID             = MustParseGUID("31f9f286-5084-42fe-b720-2b0264993763")
	zeroGUID                      GUID
)

func ParseGUID(s string) (GUID, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "active" || s == "current" {
		return GUID{}, fmt.Errorf("%q 不是具体 GUID", s)
	}
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
	return GUID{Data1: data1, Data2: uint16(data2), Data3: uint16(data3), Data4: data4}, nil
}

func MustParseGUID(s string) GUID {
	guid, err := ParseGUID(s)
	if err != nil {
		panic(err)
	}
	return guid
}

func (g GUID) String() string {
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3], g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

func (g GUID) IsZero() bool { return g == zeroGUID }

func IsHeteroThresholdGUID(g GUID) bool {
	return g == HeteroIncreaseThresholdClass1 || g == HeteroDecreaseThresholdClass1 || g == HeteroIncreaseThresholdClass2 || g == HeteroDecreaseThresholdClass2
}

func NormalizeGUIDString(s string) (string, error) {
	g, err := ParseGUID(s)
	if err != nil {
		return "", err
	}
	return g.String(), nil
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
