// +build plan9

package isatty

import (
	"syscall"
)

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
<<<<<<< HEAD
	path, err := syscall.Fd2path(int(fd))
=======
	path, err := syscall.Fd2path(fd)
>>>>>>> cbc9bb05... fixup add vendor back
	if err != nil {
		return false
	}
	return path == "/dev/cons" || path == "/mnt/term/dev/cons"
}

// IsCygwinTerminal return true if the file descriptor is a cygwin or msys2
// terminal. This is also always false on this environment.
func IsCygwinTerminal(fd uintptr) bool {
	return false
}
