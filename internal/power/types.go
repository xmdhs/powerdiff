package power

import "fmt"

type RegType uint32

const (
	RegNone RegType = iota
	RegSZ
	RegExpandSZ
	RegBinary
	RegDWORD
	RegDWORDBigEndian
	RegLink
	RegMultiSZ
	RegResourceList
	RegFullResourceDescriptor
	RegResourceRequirementsList
	RegQWORD
)

func (r RegType) String() string {
	switch r {
	case RegNone:
		return "REG_NONE"
	case RegSZ:
		return "REG_SZ"
	case RegExpandSZ:
		return "REG_EXPAND_SZ"
	case RegBinary:
		return "REG_BINARY"
	case RegDWORD:
		return "REG_DWORD"
	case RegDWORDBigEndian:
		return "REG_DWORD_BIG_ENDIAN"
	case RegLink:
		return "REG_LINK"
	case RegMultiSZ:
		return "REG_MULTI_SZ"
	case RegResourceList:
		return "REG_RESOURCE_LIST"
	case RegFullResourceDescriptor:
		return "REG_FULL_RESOURCE_DESCRIPTOR"
	case RegResourceRequirementsList:
		return "REG_RESOURCE_REQUIREMENTS_LIST"
	case RegQWORD:
		return "REG_QWORD"
	default:
		return fmt.Sprintf("REG_%d", uint32(r))
	}
}

type Scheme struct {
	GUID        string `json:"guid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active,omitempty"`
	Type        string `json:"type"`
}

type Setting struct {
	GUID           string          `json:"guid"`
	SubgroupGUID   string          `json:"subgroupGuid"`
	Subgroup       string          `json:"subgroup"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Hidden         bool            `json:"hidden"`
	IsRanged       bool            `json:"isRanged"`
	Min            *uint32         `json:"min,omitempty"`
	Max            *uint32         `json:"max,omitempty"`
	Increment      *uint32         `json:"increment,omitempty"`
	Units          string          `json:"units,omitempty"`
	PossibleValues []PossibleValue `json:"possibleValues,omitempty"`
	Values         []SettingValue  `json:"values,omitempty"`
	ReadError      string          `json:"readError,omitempty"`
	CanAddPossible bool            `json:"canAddPossibleValues,omitempty"`
}

type SettingValue struct {
	SchemeGUID string  `json:"schemeGuid"`
	AC         *uint32 `json:"ac,omitempty"`
	DC         *uint32 `json:"dc,omitempty"`
	ACDefault  *uint32 `json:"acDefault,omitempty"`
	DCDefault  *uint32 `json:"dcDefault,omitempty"`
	HasDC      bool    `json:"hasDC"`
}

type PossibleValue struct {
	Index       uint32 `json:"index"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
	RegType     string `json:"regType,omitempty"`
	RawHex      string `json:"rawHex,omitempty"`
}

type DiffItem struct {
	SettingGUID  string      `json:"settingGuid"`
	SubgroupGUID string      `json:"subgroupGuid"`
	Setting      string      `json:"setting"`
	Subgroup     string      `json:"subgroup"`
	Description  string      `json:"description,omitempty"`
	Diffs        []ValueDiff `json:"diffs,omitempty"`
	ReadErrors   []string    `json:"readErrors,omitempty"`
}

type ValueDiff struct {
	Source      string `json:"source"`
	Current     uint32 `json:"current"`
	Default     uint32 `json:"default"`
	CurrentText string `json:"currentText"`
	DefaultText string `json:"defaultText"`
}

type UpdateSettingRequest struct {
	Scheme   string  `json:"scheme"`
	Subgroup string  `json:"subgroup"`
	AC       *uint32 `json:"ac"`
	DC       *uint32 `json:"dc"`
}

type HiddenRequest struct {
	Subgroup string `json:"subgroup"`
	Hidden   bool   `json:"hidden"`
}

type PossibleValueRequest struct {
	Subgroup string `json:"subgroup"`
	Index    uint32 `json:"index"`
	RawHex   string `json:"rawHex"`
}

type ImportResult struct {
	Applied int      `json:"applied"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

type PlatformInfo struct {
	Windows bool   `json:"windows"`
	Role    string `json:"role"`
	HasDC   bool   `json:"hasDC"`
}
