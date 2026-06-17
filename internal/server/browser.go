package server

import (
	"fmt"
	"os/exec"
	"runtime"
)

func OpenBrowser(url string) error {
	var cmds [][]string
	switch runtime.GOOS {
	case "windows":
		cmds = [][]string{{"rundll32", "url.dll,FileProtocolHandler", url}, {"cmd", "/c", "start", "", url}}
	case "darwin":
		cmds = [][]string{{"open", url}}
	default:
		cmds = [][]string{{"xdg-open", url}}
	}
	var last error
	for _, item := range cmds {
		cmd := exec.Command(item[0], item[1:]...)
		if err := cmd.Start(); err != nil {
			last = err
			continue
		}
		return nil
	}
	return fmt.Errorf("打开浏览器失败: %w", last)
}
