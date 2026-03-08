//go:build windows

package filelock

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const lockfileExclusiveLock = 0x00000002

// Lock acquires an exclusive lock on the given file descriptor.
func Lock(fd uintptr) error {
	h := syscall.Handle(fd)
	ol := new(syscall.Overlapped)
	r1, _, err := procLockFileEx.Call(
		uintptr(h),
		lockfileExclusiveLock,
		0,
		1, 0,
		uintptr(unsafe.Pointer(ol)),
	)
	if r1 == 0 {
		return fmt.Errorf("LockFileEx: %w", err)
	}
	return nil
}

// Unlock releases the lock on the given file descriptor.
func Unlock(fd uintptr) error {
	h := syscall.Handle(fd)
	ol := new(syscall.Overlapped)
	r1, _, err := procUnlockFileEx.Call(
		uintptr(h),
		0,
		1, 0,
		uintptr(unsafe.Pointer(ol)),
	)
	if r1 == 0 {
		return fmt.Errorf("UnlockFileEx: %w", err)
	}
	return nil
}
