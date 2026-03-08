//go:build !windows

package filelock

import "syscall"

// Lock acquires an exclusive lock on the given file descriptor.
func Lock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_EX)
}

// Unlock releases the lock on the given file descriptor.
func Unlock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_UN)
}
