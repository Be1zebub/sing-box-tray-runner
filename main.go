//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"

	"github.com/zelgray/sing-box-tray/internal/config"
	"github.com/zelgray/sing-box-tray/internal/elevation"
	"github.com/zelgray/sing-box-tray/internal/i18n"
	"github.com/zelgray/sing-box-tray/internal/state"
	"github.com/zelgray/sing-box-tray/internal/tray"
)

const mutexName = "Global\\SingBoxTray"

func main() {
	forceMode := flag.String("force-mode", "", "override default_mode from config (used after UAC re-launch)")
	flag.Parse()

	// No config loaded yet, so fall back to OS locale detection.
	strs := i18n.Get(i18n.Detect())

	// Prevent multiple instances via a named kernel mutex.
	namePtr, _ := windows.UTF16PtrFromString(mutexName)
	h, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil {
		if err == windows.ERROR_ALREADY_EXISTS {
			os.Exit(0)
		}
		showError(fmt.Sprintf(strs.StartupErrMutexFmt, err))
		os.Exit(1)
	}
	releaseMutex := func() { windows.CloseHandle(h) }
	defer releaseMutex()

	exePath, err := os.Executable()
	if err != nil {
		showError(fmt.Sprintf(strs.StartupErrExePathFmt, err))
		os.Exit(1)
	}
	exeDir := filepath.Dir(exePath)

	cfg, err := config.Load(exeDir)
	if err != nil {
		showError(fmt.Sprintf(strs.StartupErrConfigFmt, err))
		os.Exit(1)
	}
	strs = i18n.Get(i18n.Resolve(cfg.Language))

	initialMode, _ := state.ParseMode(cfg.DefaultMode)
	if *forceMode != "" {
		if m, parseErr := state.ParseMode(*forceMode); parseErr == nil {
			initialMode = m
		}
	}

	// Elevate early when TUN is the startup mode to avoid switching later.
	if initialMode == state.ModeTUN && !elevation.IsElevated() {
		if err := elevation.RelaunchAsAdmin(fmt.Sprintf("--force-mode=%s", initialMode)); err != nil {
			showError(fmt.Sprintf(strs.StartupErrElevateFmt, err))
			os.Exit(1)
		}
		releaseMutex() // Let the elevated instance acquire the mutex immediately.
		os.Exit(0)
	}

	app := tray.NewApp(cfg, exeDir, initialMode, releaseMutex, strs)
	systray.Run(app.OnReady, app.OnExit)
}

var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	procMessageBox = user32.NewProc("MessageBoxW")
)

func showError(msg string) {
	title, _ := windows.UTF16PtrFromString("sing-box-tray")
	text, _ := windows.UTF16PtrFromString(msg)
	const mbIconError = 0x10
	procMessageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbIconError)
}
