//go:build windows

package aboutwin

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"

	"github.com/zelgray/sing-box-tray/internal/appicon"
	"github.com/zelgray/sing-box-tray/internal/i18n"
)

var (
	mu sync.Mutex
	mw *walk.MainWindow
)

// Show opens the About window, or brings it to front if already open.
func Show(strs i18n.Strings, appTitle, trayVersion, repoURL string) {
	mu.Lock()
	existing := mw
	mu.Unlock()

	if existing != nil {
		existing.Synchronize(func() {
			win.ShowWindow(existing.Handle(), win.SW_RESTORE)
			win.SetForegroundWindow(existing.Handle())
		})
		return
	}

	go runWindow(strs, appTitle, trayVersion, repoURL)
}

func runWindow(strs i18n.Strings, appTitle, trayVersion, repoURL string) {
	runtime.LockOSThread()

	var w *walk.MainWindow

	var winIcon walk.Image
	if ic := appicon.Icon(); ic != nil {
		winIcon = ic
	}

	if err := (MainWindow{
		AssignTo: &w,
		Title:    strs.MenuAbout,
		Icon:     winIcon,
		MinSize:  Size{Width: 340, Height: 130},
		Size:     Size{Width: 340, Height: 130},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 6},
		Children: []Widget{
			Label{Text: fmt.Sprintf("%s %s", appTitle, trayVersion)},
			LinkLabel{
				Text: fmt.Sprintf(`<a href="%s">%s</a>`, repoURL, repoURL),
				OnLinkActivated: func(link *walk.LinkLabelLink) {
					openURL(link.URL())
				},
			},
			VSpacer{},
			Label{Text: "MIT License"},
		},
	}).Create(); err != nil {
		return
	}

	mu.Lock()
	mw = w
	mu.Unlock()

	w.Show()
	win.SetForegroundWindow(w.Handle())
	w.Run()

	mu.Lock()
	mw = nil
	mu.Unlock()
}

var (
	shell32        = windows.NewLazySystemDLL("shell32.dll")
	procShellExecW = shell32.NewProc("ShellExecuteW")
)

// openURL launches url in the user's default browser via ShellExecuteW —
// the same mechanism internal/elevation uses for the "runas" UAC relaunch.
func openURL(url string) {
	verb, _ := windows.UTF16PtrFromString("open")
	urlPtr, _ := windows.UTF16PtrFromString(url)
	_, _, _ = procShellExecW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(urlPtr)), 0, 0, 1)
}
