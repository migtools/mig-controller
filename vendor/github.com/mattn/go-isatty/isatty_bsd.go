// +build darwin freebsd openbsd netbsd dragonfly
// +build !appengine

package isatty

<<<<<<< HEAD
import "golang.org/x/sys/unix"

// IsTerminal return true if the file descriptor is terminal.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	return err == nil
=======
import (
	"syscall"
	"unsafe"
)

const ioctlReadTermios = syscall.TIOCGETA

// IsTerminal return true if the file descriptor is terminal.
func IsTerminal(fd uintptr) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioctlReadTermios, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
>>>>>>> cbc9bb05... fixup add vendor back
}

// IsCygwinTerminal return true if the file descriptor is a cygwin or msys2
// terminal. This is also always false on this environment.
func IsCygwinTerminal(fd uintptr) bool {
	return false
}
