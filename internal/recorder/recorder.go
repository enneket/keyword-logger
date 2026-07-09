package recorder

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"keyword-logger/internal/counter"
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
		{0xFF0A, "Linefeed"},
		{0xFF0B, "Clear"},
		{0xFF0D, "Enter"},
		{0xFF13, "Pause"},
		{0xFF14, "Scroll_Lock"},
		{0xFF15, "Sys_Req"},
		{0xFF1B, "Escape"},
		{0xFF20, "Caps_Lock"},
		{0xFF21, "Shift_Lock"},
		{0xFF27, "Print"},
		{0xFF69, "Break"},
		{0xFF6A, "Mode_Switch"},
		{0xFFF0, "Delete"},
		{0xFFF1, "Insert"},
		{0xFFF2, "Help"},
		{0xFFF5, "Print"},
		{0xFFF6, "Select"},
		{0xFFF8, "Again"},
		{0xFFFB, "Front"},
		{0xFFFC, "Copy"},
		{0xFFFE, "Paste"},
		{0xFFFF, "Cut"},

		// 导航键
		{0xFF50, "Home"},
		{0xFF51, "Left"},
		{0xFF52, "Up"},
		{0xFF53, "Right"},
		{0xFF54, "Down"},
		{0xFF55, "Page_Up"},
		{0xFF56, "Page_Down"},
		{0xFF57, "End"},
		{0xFF58, "Begin"},
		{0xFF60, "Insert"},
		{0xFF61, "Undo"},
		{0xFF62, "Redo"},
		{0xFF63, "Find"},
		{0xFF65, "Cancel"},
		{0xFF67, "Menu"},
		{0xFF7D, "KP_Home"},
		{0xFF7E, "KP_Left"},
		{0xFF7F, "KP_Begin"},
		{0xFF80, "KP_Right"},
		{0xFF81, "KP_End"},
		{0xFF82, "KP_Up"},
		{0xFF83, "KP_Page_Up"},
		{0xFF84, "KP_Down"},
		{0xFF85, "KP_Page_Down"},
		{0xFF86, "KP_Insert"},
		{0xFF87, "KP_Delete"},

		// 功能键 (F1-F24)
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
		{0xFFCA, "F13"},
		{0xFFCB, "F14"},
		{0xFFCC, "F15"},
		{0xFFCD, "F16"},
		{0xFFCE, "F17"},
		{0xFFCF, "F18"},
		{0xFFD0, "F19"},
		{0xFFD1, "F20"},
		{0xFFD2, "F21"},
		{0xFFD3, "F22"},
		{0xFFD4, "F23"},
		{0xFFD5, "F24"},
		{0xFFB4, "KP_F1"},
		{0xFFB5, "KP_F2"},
		{0xFFB6, "KP_F3"},
		{0xFFB7, "KP_F4"},
		{0xFF86, "KP_F1"},
		{0xFF87, "KP_F2"},
		{0xFF88, "KP_F3"},
		{0xFF89, "KP_F4"},
		{0xFF8B, "KP_F1"},
		{0xFF8C, "KP_F2"},
		{0xFF8D, "KP_F3"},
		{0xFF8E, "KP_F4"},
		{0xFF9B, "KP_F1"},
		{0xFF9C, "KP_F2"},
		{0xFF9D, "KP_F3"},
		{0xFF9E, "KP_F4"},
		{0xFF9F, "KP_F5"},

		// 数字键盘
		{0xFF7F, "Num_Lock"},
		{0xFF80, "KP_Space"},
		{0xFF85, "KP_Tab"},
		{0xFF89, "KP_Multiply"},
		{0xFF8A, "KP_Add"},
		{0xFF8B, "KP_Separator"},
		{0xFF8C, "KP_Subtract"},
		{0xFF8D, "KP_Decimal"},
		{0xFF8E, "KP_Divide"},
		{0xFF9D, "KP_Equal"},
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
		{0xFF9A, "KP_Equal"},

		// 修饰键
		{0xFFE1, "Shift_L"},
		{0xFFE2, "Shift_R"},
		{0xFFE3, "Control_L"},
		{0xFFE4, "Control_R"},
		{0xFFE5, "Caps_Lock"},
		{0xFFE6, "Shift_Lock"},
		{0xFFE7, "Meta_L"},
		{0xFFE8, "Meta_R"},
		{0xFFE9, "Alt_L"},
		{0xFFEA, "Alt_R"},
		{0xFFEB, "Super_L"},
		{0xFFEC, "Super_R"},
		{0xFFED, "Hyper_L"},
		{0xFFEE, "Hyper_R"},

		// 音频控制
		{0xFFB8, "XF86AudioMute"},
		{0xFFB9, "XF86AudioLowerVolume"},
		{0xFFBA, "XF86AudioRaiseVolume"},
		{0xFFBB, "XF86AudioPlay"},
		{0xFFBC, "XF86AudioStop"},
		{0xFFBD, "XF86AudioPrev"},
		{0xFFBE, "XF86AudioNext"},
		{0xFFBF, "XF86AudioRecord"},
		{0xFFC0, "XF86AudioPause"},
		{0xFFC1, "XF86AudioRewind"},
		{0xFFC2, "XF86AudioFastForward"},
		{0xFFC3, "XF86HomePage"},
		{0xFFC4, "XF86Mail"},
		{0xFFC5, "XF86Start"},
		{0xFFC6, "XF86Search"},
		{0xFFC7, "XF86AudioMedia"},
		{0xFFC8, "XF86Sleep"},
		{0xFFC9, "XF86WakeUp"},
		{0xFFCA, "XF86PowerOff"},
		{0xFFCB, "XF86Suspend"},
		{0xFFCC, "XF86LCDBrightness"},
		{0xFFCD, "XF86Battery"},
		{0xFFD0, "XF86Launch1"},
		{0xFFD1,  "XF86Launch2"},
		{0xFFD2,  "XF86Launch3"},
		{0xFFD3,  "XF86Launch4"},
		{0xFFD4,  "XF86Launch5"},
		{0xFFD5,  "XF86Launch6"},
		{0xFFD6,  "XF86Launch7"},
		{0xFFD7,  "XF86Launch8"},
		{0xFFD8,  "XF86Launch9"},
		{0xFFD9,  "XF86LaunchA"},
		{0xFFDA,  "XF86LaunchB"},
		{0xFFDB,  "XF86LaunchC"},
		{0xFFDC,  "XF86LaunchD"},
		{0xFFDD,  "XF86LaunchE"},
		{0xFFDE,  "XF86LaunchF"},
		{0xFFDF,  "XF86LaunchG"},
		{0xFFE0,  "XF86LaunchH"},

		// 其他
		{0xFF31, "F1"},
		{0xFF32, "F2"},
		{0xFF33, "F3"},
		{0xFF34, "F4"},
		{0xFF35, "F5"},
		{0xFF36, "F6"},
		{0xFF37, "F7"},
		{0xFF38, "F8"},
		{0xFF39, "F9"},
		{0xFF3A, "F10"},
		{0xFF3B, "F11"},
		{0xFF3C, "F12"},
		{0xFF3D, "F13"},
		{0xFF3E,  "F14"},
		{0xFF3F,  "F15"},
		{0xFF40,  "F16"},
		{0xFF41,  "F17"},
		{0xFF42,  "F18"},
		{0xFF43,  "F19"},
		{0xFF44,  "F20"},
		{0xFF45,  "F21"},
		{0xFF46,  "F22"},
		{0xFF47,  "F23"},
		{0xFF48,  "F24"},
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
	mu       sync.Mutex
	counter  *counter.Counter
	tracker  *window.Tracker
	stopCh   chan struct{}
	km       *keycodeMapper
	display  string
}

func New(c *counter.Counter, t *window.Tracker) *Recorder {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}
	return &Recorder{
		counter: c,
		tracker: t,
		stopCh:  make(chan struct{}),
		km:      newKeycodeMapper(),
		display: display,
	}
}

func (r *Recorder) Start() error {
	r.km.refresh()

	go r.inputLoop()
	return nil
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
		r.counter.Increment(app, keyName)
	}
}

func (r *Recorder) Stop() {
	close(r.stopCh)
}
