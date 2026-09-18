//go:build windows

package logwin

// TODO: lxn/walk requires a Windows manifest referencing Common Controls v6
// for proper visual styling. Without it the window works but looks like
// Windows XP style. To add it: install the `rsrc` tool, create app.manifest,
// run `go generate`, and commit the generated rsrc.syso.

import (
	"runtime"
	"strings"
	"sync"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"github.com/Be1zebub/sing-box-tray-runner/internal/appicon"
	"github.com/Be1zebub/sing-box-tray-runner/internal/i18n"
	"github.com/Be1zebub/sing-box-tray-runner/internal/logbuf"
)

var (
	mu sync.Mutex
	mw *walk.MainWindow
)

// Show opens the log viewer window, or brings it to front if already open.
func Show(buf *logbuf.Buffer, maxLines int, strs i18n.Strings) {
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

	go runWindow(buf, maxLines, strs)
}

func runWindow(buf *logbuf.Buffer, maxLines int, strs i18n.Strings) {
	runtime.LockOSThread()

	var w *walk.MainWindow
	var te *walk.TextEdit

	var winIcon walk.Image
	if ic := appicon.Icon(); ic != nil {
		winIcon = ic
	}

	if err := (MainWindow{
		AssignTo: &w,
		Title:    strs.LogWindowTitle,
		Icon:     winIcon,
		MinSize:  Size{Width: 700, Height: 400},
		Size:     Size{Width: 900, Height: 500},
		Layout:   VBox{MarginsZero: true},
		Children: []Widget{
			TextEdit{
				AssignTo: &te,
				ReadOnly: true,
				VScroll:  true,
				HScroll:  true,
				Font:     Font{Family: "Consolas", PointSize: 9},
			},
		},
	}).Create(); err != nil {
		buf.Append("[logwin] failed to create window: " + err.Error())
		return
	}

	mu.Lock()
	mw = w
	mu.Unlock()

	setText(te, buf.Lines())
	w.Show()
	win.SetForegroundWindow(w.Handle())

	sub := buf.Subscribe()
	go func() {
		for range sub {
			w.Synchronize(func() {
				setText(te, buf.Lines())
			})
		}
	}()

	w.Run()

	mu.Lock()
	mw = nil
	mu.Unlock()
}

func setText(te *walk.TextEdit, lines []string) {
	_ = te.SetText(strings.Join(lines, "\r\n"))
	scrollToBottom(te)
}

func scrollToBottom(te *walk.TextEdit) {
	const wmVScroll = 0x0115
	const sbBottom = 7
	win.SendMessage(te.Handle(), wmVScroll, sbBottom, 0)
}
