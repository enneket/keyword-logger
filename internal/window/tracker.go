package window

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

type Tracker struct {
	conn       *xgb.Conn
	activeAtom xproto.Atom
	pidAtom    xproto.Atom
	classAtom  xproto.Atom
	currentApp string
}

func New() (*Tracker, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("x11 connect: %w", err)
	}

	activeAtom, err := connInternAtom(conn, "_NET_ACTIVE_WINDOW")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("intern _NET_ACTIVE_WINDOW: %w", err)
	}

	pidAtom, err := connInternAtom(conn, "_NET_WM_PID")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("intern _NET_WM_PID: %w", err)
	}

	classAtom, err := connInternAtom(conn, "WM_CLASS")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("intern WM_CLASS: %w", err)
	}

	return &Tracker{
		conn:       conn,
		activeAtom: activeAtom,
		pidAtom:    pidAtom,
		classAtom:  classAtom,
	}, nil
}

func connInternAtom(conn *xgb.Conn, name string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return reply.Atom, nil
}

func (t *Tracker) Refresh() {
	win, err := t.getActiveWindow()
	if err != nil {
		return
	}
	if win == 0 {
		return
	}

	app := t.getAppName(win)
	if app != "" {
		t.currentApp = app
	}
}

func (t *Tracker) CurrentApp() string {
	return t.currentApp
}

func (t *Tracker) getActiveWindow() (xproto.Window, error) {
	root := xproto.Setup(t.conn).DefaultScreen(t.conn).Root

	reply, err := xproto.GetProperty(t.conn, false, root,
		t.activeAtom, xproto.AtomWindow, 0, 1).Reply()
	if err != nil {
		return 0, err
	}
	if reply.Format != 32 || len(reply.Value) < 4 {
		return 0, nil
	}

	return xproto.Window(xgb.Get32(reply.Value)), nil
}

func (t *Tracker) getAppName(win xproto.Window) string {
	app := t.getByPID(win)
	if app != "" {
		return app
	}
	return t.getByClass(win)
}

func (t *Tracker) getByPID(win xproto.Window) string {
	reply, err := xproto.GetProperty(t.conn, false, win,
		t.pidAtom, xproto.AtomCardinal, 0, 1).Reply()
	if err != nil || reply.Format != 32 || len(reply.Value) < 4 {
		return ""
	}

	pid := xgb.Get32(reply.Value)
	comm, err := os.ReadFile(filepath.Join("/proc", fmt.Sprint(pid), "comm"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(comm))
}

func (t *Tracker) getByClass(win xproto.Window) string {
	reply, err := xproto.GetProperty(t.conn, false, win,
		t.classAtom, xproto.AtomString, 0, 1024).Reply()
	if err != nil || reply.Format != 8 || len(reply.Value) == 0 {
		return ""
	}

	parts := strings.SplitN(string(reply.Value), "\x00", 2)
	if len(parts) < 2 {
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(parts[1])
}

func (t *Tracker) Close() {
	t.conn.Close()
}

func (t *Tracker) Run(interval time.Duration, stopCh <-chan struct{}) {
	t.Refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.Refresh()
		case <-stopCh:
			return
		}
	}
}
