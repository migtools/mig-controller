<<<<<<< HEAD
// +build linux
// +build arm64 amd64 mips mipsle mips64 mips64le ppc ppc64 ppc64le riscv64 s390x
=======
// +build linux,arm64 linux,amd64 linux,ppc linux,ppc64 linux,ppc64le linux,s390x
>>>>>>> cbc9bb05... fixup add vendor back

package system

import (
<<<<<<< HEAD
	"golang.org/x/sys/unix"
=======
	"syscall"
>>>>>>> cbc9bb05... fixup add vendor back
)

// Setuid sets the uid of the calling thread to the specified uid.
func Setuid(uid int) (err error) {
<<<<<<< HEAD
	_, _, e1 := unix.RawSyscall(unix.SYS_SETUID, uintptr(uid), 0, 0)
=======
	_, _, e1 := syscall.RawSyscall(syscall.SYS_SETUID, uintptr(uid), 0, 0)
>>>>>>> cbc9bb05... fixup add vendor back
	if e1 != 0 {
		err = e1
	}
	return
}

// Setgid sets the gid of the calling thread to the specified gid.
func Setgid(gid int) (err error) {
<<<<<<< HEAD
	_, _, e1 := unix.RawSyscall(unix.SYS_SETGID, uintptr(gid), 0, 0)
=======
	_, _, e1 := syscall.RawSyscall(syscall.SYS_SETGID, uintptr(gid), 0, 0)
>>>>>>> cbc9bb05... fixup add vendor back
	if e1 != 0 {
		err = e1
	}
	return
}
