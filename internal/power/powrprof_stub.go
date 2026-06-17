//go:build !windows

package power

import "errors"

var errWindowsOnly = errors.New("Windows power APIs are only available on Windows")

func Platform() PlatformInfo                   { return PlatformInfo{Windows: false, Role: "unsupported", HasDC: false} }
func ListSchemes() ([]Scheme, error)           { return nil, errWindowsOnly }
func ActiveScheme() (Scheme, error)            { return Scheme{}, errWindowsOnly }
func SetActiveScheme(_ string) error           { return errWindowsOnly }
func ListOverlays() ([]Scheme, error)          { return []Scheme{}, nil }
func SetActiveOverlay(_ string) error          { return errWindowsOnly }
func ListSettings(_ string) ([]Setting, error) { return nil, errWindowsOnly }
func UpdateSetting(_ string, _ UpdateSettingRequest) (SettingValue, error) {
	return SettingValue{}, errWindowsOnly
}
func SetHidden(_ string, _ HiddenRequest) error               { return errWindowsOnly }
func AddPossibleValue(_ string, _ PossibleValueRequest) error { return errWindowsOnly }
func DiffScheme(_ string, _ bool) ([]DiffItem, error)         { return nil, errWindowsOnly }
func DiffAgainst(_, _ string) ([]DiffItem, error)             { return nil, errWindowsOnly }
func ExportXML(_ string) ([]byte, error)                      { return nil, errWindowsOnly }
func ImportXML(_ []byte) (ImportResult, error)                { return ImportResult{}, errWindowsOnly }
func ExportScript(_ string) (string, error)                   { return "", errWindowsOnly }
