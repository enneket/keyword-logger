package recorder

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"keyword-logger/internal/counter"
	"keyword-logger/internal/persist"
	"keyword-logger/internal/window"
)

type keyMapEntry struct {
	keysym uint32
	name   string
}

var keyNameMap []keyMapEntry

func init() {
	keyNameMap = []keyMapEntry{
		// 控制键
		{0xFF08, "BackSpace"},
		{0xFF09, "Tab"},
		{0xFF0D, "Enter"},
		{0xFF0A, "Linefeed"},
		{0xFF0B, "Clear"},
		{0xFF1B, "Escape"},
		{0xFF13, "Pause"},
		{0xFF14, "Scroll_Lock"},
		{0xFF15, "Sys_Req"},
		{0xFF20, "Caps_Lock"},
		{0xFF27, "Print"},
		{0xFF69, "Break"},

		// 导航键
		{0xFF50, "Home"},
		{0xFF51, "Left"},
		{0xFF52, "Up"},
		{0xFF53, "Right"},
		{0xFF54, "Down"},
		{0xFF55, "Page_Up"},
		{0xFF56, "Page_Down"},
		{0xFF57, "End"},
		{0xFFF0, "Delete"},
		{0xFFF1, "Insert"},

		// 功能键 F1-F12
		{0xFFBE, "F1"},
		{0xFFBF, "F2"},
		{0xFFC0, "F3"},
		{0xFFC1, "F4"},
		{0xFFC2, "F5"},
		{0xFFC3, "F6"},
		{0xFFC4, "F7"},
		{0xFFC5, "F8"},
		{0xFFC6, "F9"},
		{0xFFC7, "F10"},
		{0xFFC8, "F11"},
		{0xFFC9, "F12"},

		// 数字小键盘
		{0xFF7F, "Num_Lock"},
		{0xFFAA, "KP_0"},
		{0xFFAB, "KP_1"},
		{0xFFAC, "KP_2"},
		{0xFFAD, "KP_3"},
		{0xFFAE, "KP_4"},
		{0xFFAF, "KP_5"},
		{0xFFB0, "KP_6"},
		{0xFFB1, "KP_7"},
		{0xFFB2, "KP_8"},
		{0xFFB3, "KP_9"},
		{0xFF89, "KP_Multiply"},
		{0xFF8A, "KP_Add"},
		{0xFF8B, "KP_Separator"},
		{0xFF8C, "KP_Subtract"},
		{0xFF8D, "KP_Decimal"},
		{0xFF8E, "KP_Divide"},
		{0xFF7E, "KP_Enter"},

		// 修饰键
		{0xFFE1, "Shift_L"},
		{0xFFE2, "Shift_R"},
		{0xFFE3, "Control_L"},
		{0xFFE4, "Control_R"},
		{0xFFE9, "Alt_L"},
		{0xFFEA, "Alt_R"},
		{0xFFEB, "Super_L"},
		{0xFFEC, "Super_R"},
		{0xFFE7, "Meta_L"},
		{0xFFE8, "Meta_R"},
	}
}

func keysymToName(ks uint32) string {
	if ks >= 0x20 && ks <= 0x7E {
		return string(rune(ks))
	}
	for _, e := range keyNameMap {
		if e.keysym == ks {
			return e.name
		}
	}
	return fmt.Sprintf("0x%X", ks)
}

type keycodeMapper struct {
	mu       sync.RWMutex
	firstKc  byte
	countKc  byte
	perKey   byte
	keysyms  []uint32
}

func newKeycodeMapper() *keycodeMapper {
	return &keycodeMapper{}
}

func (km *keycodeMapper) refresh() {
	cmd := exec.Command("xmodmap", "-pk")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	km.firstKc = 8
	km.countKc = 248 - 8 + 1
	km.perKey = 1
	km.keysyms = make([]uint32, km.countKc)

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		var kc byte
		if _, err := fmt.Sscanf(parts[0], "%d", &kc); err != nil {
			continue
		}
		if kc < km.firstKc || int(kc) >= int(km.firstKc)+int(km.countKc) {
			continue
		}

		idx := int(kc - km.firstKc)

		if len(parts) < 2 || parts[1] == "" {
			continue
		}

		var ks uint32
		if _, err := fmt.Sscanf(parts[1], "0x%x", &ks); err != nil {
			continue
		}
		km.keysyms[idx] = ks
	}
}

func (km *keycodeMapper) keycodeToName(kc byte) string {
	km.mu.RLock()
	defer km.mu.RUnlock()

	if int(kc) < int(km.firstKc) || int(kc) >= int(km.firstKc)+int(km.countKc) {
		return fmt.Sprintf("KC%d", kc)
	}

	idx := int(kc - km.firstKc)
	if idx >= len(km.keysyms) {
		return fmt.Sprintf("KC%d", kc)
	}

	ks := km.keysyms[idx]
	return keysymToName(ks)
}

type Recorder struct {
	mu          sync.Mutex
	counter     *counter.Counter
	tracker     *window.Tracker
	stopCh      chan struct{}
	km          *keycodeMapper
	display     string
	saver       *persist.Saver

	// 防抖合并：累积窗口内的按键，批量写入
	pending     map[string]map[string]int64
}

const flushInterval = 100 * time.Millisecond

func New(c *counter.Counter, t *window.Tracker, saver *persist.Saver) *Recorder {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}
	return &Recorder{
		counter:  c,
		tracker:  t,
		saver:    saver,
		stopCh:   make(chan struct{}),
		km:       newKeycodeMapper(),
		display:  display,
		pending:  make(map[string]map[string]int64),
	}
}

func (r *Recorder) Start() error {
	r.km.refresh()
	go r.flushLoop()
	go r.inputLoop()
	return nil
}

func (r *Recorder) flushLoop() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.flush()
		case <-r.stopCh:
			r.flush()
			return
		}
	}
}

func (r *Recorder) flush() {
	r.mu.Lock()
	if len(r.pending) == 0 {
		r.mu.Unlock()
		return
	}
	toFlush := r.pending
	r.pending = make(map[string]map[string]int64)
	r.mu.Unlock()

	r.counter.Merge(toFlush)
	// 同步持久化本批按键，避免 Saver 全量 snapshot 重复叠加
	if r.saver != nil {
		r.saver.SaveBatch(toFlush)
	}
}

func (r *Recorder) record(app, keyName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending[app] == nil {
		r.pending[app] = make(map[string]int64)
	}
	r.pending[app][keyName]++
}

func (r *Recorder) findKeyboardDevices() ([]int, error) {
	cmd := exec.Command("xinput", "list")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var ids []int
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "slave  keyboard") {
			continue
		}
		if strings.Contains(line, "XTEST") || strings.Contains(line, "Power Button") || strings.Contains(line, "Video Bus") || strings.Contains(line, "extra buttons") || strings.Contains(line, "System Control") || strings.Contains(line, "Consumer Control") {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(line, "    ↳ %*s %*s %*s %*s	id=%d", &id); err == nil {
			ids = append(ids, id)
			continue
		}
		parts := strings.Split(line, "id=")
		if len(parts) >= 2 {
			if _, err := fmt.Sscanf(parts[1], "%d", &id); err == nil {
				ids = append(ids, id)
			}
		}
	}

	if len(ids) == 0 {
		return nil, errors.New("no suitable keyboard device found")
	}
	return ids, nil
}

func (r *Recorder) inputLoop() {
	deviceIDs, err := r.findKeyboardDevices()
	if err != nil {
		os.Stderr.WriteString("recorder: " + err.Error() + "\n")
		return
	}

	var wg sync.WaitGroup
	for _, deviceID := range deviceIDs {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.watchDevice(id)
		}(deviceID)
	}

	<-r.stopCh
	wg.Wait()
}

func (r *Recorder) watchDevice(deviceID int) {
	cmd := exec.Command("xinput", "test", fmt.Sprintf("%d", deviceID))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("recorder[%d]: pipe: %v\n", deviceID, err))
		return
	}

	if err := cmd.Start(); err != nil {
		os.Stderr.WriteString(fmt.Sprintf("recorder[%d]: start: %v\n", deviceID, err))
		return
	}

	defer cmd.Wait()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "key press") {
			continue
		}

		var kc byte
		if _, err := fmt.Sscanf(line, "key press %d", &kc); err != nil {
			continue
		}
		if kc == 0 {
			continue
		}

		r.tracker.Refresh()
		app := r.tracker.CurrentApp()
		if app == "" {
			app = "unknown"
		}

		keyName := r.km.keycodeToName(kc)
		r.record(app, keyName)
	}
}

func (r *Recorder) Stop() {
	close(r.stopCh)
	r.flush()
}
