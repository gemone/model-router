package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenURL opens a URL in the default browser
func OpenURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "linux":
		// Try xdg-open first, then fall back to browser commands
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", url)
		} else if _, err := exec.LookPath("x-www-browser"); err == nil {
			cmd = exec.Command("x-www-browser", url)
		} else if _, err := exec.LookPath("gnome-open"); err == nil {
			cmd = exec.Command("gnome-open", url)
		} else {
			return fmt.Errorf("no browser found (tried xdg-open, x-www-browser, gnome-open)")
		}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
