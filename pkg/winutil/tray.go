package winutil

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	user32               = syscall.NewLazyDLL("user32.dll")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procSetForeground    = user32.NewProc("SetForegroundWindow")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procLoadIconW        = user32.NewProc("LoadIconW")
)

const (
	NIM_ADD        = 0x00000000
	NIM_MODIFY     = 0x00000001
	NIM_DELETE     = 0x00000002
	NIF_MESSAGE    = 0x00000001
	NIF_ICON       = 0x00000002
	NIF_TIP        = 0x00000004
	WM_USER        = 0x0400
	WM_TRAYICON    = WM_USER + 101
	WM_COMMAND     = 0x0111
	WM_LBUTTONUP   = 0x0202
	WM_LBUTTONDBL  = 0x0203
	WM_RBUTTONUP   = 0x0205
	MF_STRING      = 0x00000000
	MF_SEPARATOR   = 0x00000800
	TPM_RIGHTALIGN = 0x0008
	TPM_BOTTOMALIGN = 0x0020

	IDM_OPEN       = 1001
	IDM_TOGGLE     = 1002
	IDM_EXIT       = 1003
)

type point struct {
	x, y int32
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             syscall.Handle
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            syscall.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uTimeoutOrVer    uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     syscall.Handle
}

type TrayManager struct {
	hwnd       syscall.Handle
	nid        notifyIconData
	onShow     func()
	onToggle   func()
	onQuit     func()
	mu         sync.Mutex
	stopChan   chan struct{}
}

var globalTray *TrayManager

func trayWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == WM_TRAYICON {
		switch lParam {
		case WM_LBUTTONUP, WM_LBUTTONDBL:
			if globalTray != nil && globalTray.onShow != nil {
				go globalTray.onShow()
			}
			return 0
		case WM_RBUTTONUP:
			if globalTray != nil {
				globalTray.showContextMenu()
			}
			return 0
		}
	} else if msg == WM_COMMAND {
		cmdID := int(wParam & 0xffff)
		switch cmdID {
		case IDM_OPEN:
			if globalTray != nil && globalTray.onShow != nil {
				go globalTray.onShow()
			}
		case IDM_TOGGLE:
			if globalTray != nil && globalTray.onToggle != nil {
				go globalTray.onToggle()
			}
		case IDM_EXIT:
			if globalTray != nil && globalTray.onQuit != nil {
				go globalTray.onQuit()
			}
		}
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func (t *TrayManager) showContextMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}

	openText, _ := syscall.UTF16PtrFromString("Show FoxyVPN")
	toggleText, _ := syscall.UTF16PtrFromString("Toggle Connect / Disconnect")
	exitText, _ := syscall.UTF16PtrFromString("Exit FoxyVPN")

	procAppendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(IDM_OPEN), uintptr(unsafe.Pointer(openText)))
	procAppendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(IDM_TOGGLE), uintptr(unsafe.Pointer(toggleText)))
	procAppendMenuW.Call(hMenu, uintptr(MF_SEPARATOR), 0, 0)
	procAppendMenuW.Call(hMenu, uintptr(MF_STRING), uintptr(IDM_EXIT), uintptr(unsafe.Pointer(exitText)))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForeground.Call(uintptr(t.hwnd))
	procTrackPopupMenu.Call(hMenu, uintptr(TPM_RIGHTALIGN|TPM_BOTTOMALIGN), uintptr(pt.x), uintptr(pt.y), 0, uintptr(t.hwnd), 0)
}

// StartTray initializes the system tray icon and message loop.
func StartTray(tooltip string, onShow, onToggle, onQuit func()) (*TrayManager, error) {
	tm := &TrayManager{
		onShow:   onShow,
		onToggle: onToggle,
		onQuit:   onQuit,
		stopChan: make(chan struct{}),
	}
	globalTray = tm

	readyChan := make(chan error, 1)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		className, _ := syscall.UTF16PtrFromString("FoxyVPNTrayClass")
		wndProcCallback := syscall.NewCallback(trayWndProc)

		wc := wndClassEx{
			cbSize:      uint32(unsafe.Sizeof(wndClassEx{})),
			lpfnWndProc: wndProcCallback,
			lpszClassName: className,
		}

		atom, _, errReg := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		if atom == 0 {
			// If already registered, that's fine
			_ = errReg
		}

		windowName, _ := syscall.UTF16PtrFromString("FoxyVPNTrayWindow")
		hwnd, _, errCreate := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(windowName)),
			0,
			0, 0, 0, 0,
			0, 0, 0, 0,
		)
		if hwnd == 0 {
			readyChan <- errCreate
			return
		}
		tm.hwnd = syscall.Handle(hwnd)

		// Load standard application icon
		hIcon, _, _ := procLoadIconW.Call(0, uintptr(32512)) // IDI_APPLICATION = 32512

		nid := notifyIconData{
			cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
			hWnd:             tm.hwnd,
			uID:              1,
			uFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
			uCallbackMessage: WM_TRAYICON,
			hIcon:            syscall.Handle(hIcon),
		}

		tipChars, _ := syscall.UTF16FromString(tooltip)
		copy(nid.szTip[:], tipChars)

		res, _, _ := procShellNotifyIconW.Call(uintptr(NIM_ADD), uintptr(unsafe.Pointer(&nid)))
		if res == 0 {
			// Try modify if already present
			procShellNotifyIconW.Call(uintptr(NIM_MODIFY), uintptr(unsafe.Pointer(&nid)))
		}
		tm.nid = nid
		readyChan <- nil

		// Message Pump for tray events
		var msg struct {
			hwnd    syscall.Handle
			message uint32
			wParam  uintptr
			lParam  uintptr
			time    uint32
			pt      point
		}

		for {
			select {
			case <-tm.stopChan:
				procShellNotifyIconW.Call(uintptr(NIM_DELETE), uintptr(unsafe.Pointer(&tm.nid)))
				procDestroyWindow.Call(uintptr(tm.hwnd))
				return
			default:
				r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
				if int32(r) <= 0 {
					return
				}
				procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
				procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
			}
		}
	}()

	err := <-readyChan
	return tm, err
}

// UpdateTooltip updates the text displayed on tray hover.
func (t *TrayManager) UpdateTooltip(tip string) {
	if t == nil || t.hwnd == 0 {
		return
	}
	tipChars, _ := syscall.UTF16FromString(tip)
	copy(t.nid.szTip[:], tipChars)
	procShellNotifyIconW.Call(uintptr(NIM_MODIFY), uintptr(unsafe.Pointer(&t.nid)))
}

// Close removes the icon from the system tray and destroys the window.
func (t *TrayManager) Close() {
	if t != nil && t.hwnd != 0 {
		close(t.stopChan)
		procPostQuitMessage.Call(0)
	}
}
