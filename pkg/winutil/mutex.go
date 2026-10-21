package winutil

import (
	"errors"
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
)

const (
	ERROR_ALREADY_EXISTS = 183
	MutexName            = "Global\\FoxyVPN_SingleInstance_Mutex"
)

type SingleInstanceLock struct {
	handle syscall.Handle
}

// AcquireSingleInstance attempts to acquire the system-wide application mutex.
func AcquireSingleInstance() (*SingleInstanceLock, error) {
	namePtr, err := syscall.UTF16PtrFromString(MutexName)
	if err != nil {
		return nil, err
	}

	h, _, errCall := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if h == 0 {
		return nil, errors.New("failed creating single instance mutex")
	}

	if errno, ok := errCall.(syscall.Errno); ok && errno == ERROR_ALREADY_EXISTS {
		_ = syscall.CloseHandle(syscall.Handle(h))
		return nil, errors.New("another instance of FoxyVPN is already running")
	}

	return &SingleInstanceLock{handle: syscall.Handle(h)}, nil
}

// Release releases the single-instance mutex.
func (l *SingleInstanceLock) Release() {
	if l != nil && l.handle != 0 {
		_ = syscall.CloseHandle(l.handle)
		l.handle = 0
	}
}
