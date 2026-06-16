//go:build windows

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// maybeInstall copies the running binary to %APPDATA%\WifiSync\WifiSync.exe
// if not already running from that location, then launches the installed copy
// and exits. This makes the downloaded .exe self-installing on first run.
// Running a newer download from any other location acts as an in-place update.
func maybeInstall() {
	currentExe, err := os.Executable()
	if err != nil {
		return
	}
	currentExe, _ = filepath.EvalSymlinks(currentExe)

	base := os.Getenv("APPDATA")
	if base == "" {
		return
	}
	installDir := filepath.Join(base, "WifiSync")
	installExe := filepath.Join(installDir, "WifiSync.exe")

	if strings.EqualFold(filepath.Clean(currentExe), filepath.Clean(installExe)) {
		return // already running from the installed location
	}

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return // fall through and run in-place
	}
	if err := installCopy(currentExe, installExe); err != nil {
		return // copy failed; fall through and run in-place
	}

	cmd := exec.Command(installExe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if cmd.Start() == nil {
		os.Exit(0)
	}
}

// installCopy writes src to dst via a temp file + rename so a failed copy
// never leaves a partial or corrupt executable at the destination.
func installCopy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, dst)
}
