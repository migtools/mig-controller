// +build linux

package system

import (
<<<<<<< HEAD
	"os"
	"os/exec"
	"sync"
	"unsafe"

	"github.com/opencontainers/runc/libcontainer/user"
	"golang.org/x/sys/unix"
)

=======
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// If arg2 is nonzero, set the "child subreaper" attribute of the
// calling process; if arg2 is zero, unset the attribute.  When a
// process is marked as a child subreaper, all of the children
// that it creates, and their descendants, will be marked as
// having a subreaper.  In effect, a subreaper fulfills the role
// of init(1) for its descendant processes.  Upon termination of
// a process that is orphaned (i.e., its immediate parent has
// already terminated) and marked as having a subreaper, the
// nearest still living ancestor subreaper will receive a SIGCHLD
// signal and be able to wait(2) on the process to discover its
// termination status.
const PR_SET_CHILD_SUBREAPER = 36

>>>>>>> cbc9bb05... fixup add vendor back
type ParentDeathSignal int

func (p ParentDeathSignal) Restore() error {
	if p == 0 {
		return nil
	}
	current, err := GetParentDeathSignal()
	if err != nil {
		return err
	}
	if p == current {
		return nil
	}
	return p.Set()
}

func (p ParentDeathSignal) Set() error {
	return SetParentDeathSignal(uintptr(p))
}

func Execv(cmd string, args []string, env []string) error {
	name, err := exec.LookPath(cmd)
	if err != nil {
		return err
	}

<<<<<<< HEAD
	return unix.Exec(name, args, env)
}

func Prlimit(pid, resource int, limit unix.Rlimit) error {
	_, _, err := unix.RawSyscall6(unix.SYS_PRLIMIT64, uintptr(pid), uintptr(resource), uintptr(unsafe.Pointer(&limit)), uintptr(unsafe.Pointer(&limit)), 0, 0)
=======
	return syscall.Exec(name, args, env)
}

func Prlimit(pid, resource int, limit syscall.Rlimit) error {
	_, _, err := syscall.RawSyscall6(syscall.SYS_PRLIMIT64, uintptr(pid), uintptr(resource), uintptr(unsafe.Pointer(&limit)), uintptr(unsafe.Pointer(&limit)), 0, 0)
>>>>>>> cbc9bb05... fixup add vendor back
	if err != 0 {
		return err
	}
	return nil
}

func SetParentDeathSignal(sig uintptr) error {
<<<<<<< HEAD
	if err := unix.Prctl(unix.PR_SET_PDEATHSIG, sig, 0, 0, 0); err != nil {
=======
	if _, _, err := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_PDEATHSIG, sig, 0); err != 0 {
>>>>>>> cbc9bb05... fixup add vendor back
		return err
	}
	return nil
}

func GetParentDeathSignal() (ParentDeathSignal, error) {
	var sig int
<<<<<<< HEAD
	if err := unix.Prctl(unix.PR_GET_PDEATHSIG, uintptr(unsafe.Pointer(&sig)), 0, 0, 0); err != nil {
=======
	_, _, err := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_GET_PDEATHSIG, uintptr(unsafe.Pointer(&sig)), 0)
	if err != 0 {
>>>>>>> cbc9bb05... fixup add vendor back
		return -1, err
	}
	return ParentDeathSignal(sig), nil
}

func SetKeepCaps() error {
<<<<<<< HEAD
	if err := unix.Prctl(unix.PR_SET_KEEPCAPS, 1, 0, 0, 0); err != nil {
=======
	if _, _, err := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_KEEPCAPS, 1, 0); err != 0 {
>>>>>>> cbc9bb05... fixup add vendor back
		return err
	}

	return nil
}

func ClearKeepCaps() error {
<<<<<<< HEAD
	if err := unix.Prctl(unix.PR_SET_KEEPCAPS, 0, 0, 0, 0); err != nil {
=======
	if _, _, err := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_KEEPCAPS, 0, 0); err != 0 {
>>>>>>> cbc9bb05... fixup add vendor back
		return err
	}

	return nil
}

func Setctty() error {
<<<<<<< HEAD
	if err := unix.IoctlSetInt(0, unix.TIOCSCTTY, 0); err != nil {
=======
	if _, _, err := syscall.RawSyscall(syscall.SYS_IOCTL, 0, uintptr(syscall.TIOCSCTTY), 0); err != 0 {
>>>>>>> cbc9bb05... fixup add vendor back
		return err
	}
	return nil
}

<<<<<<< HEAD
var (
	inUserNS bool
	nsOnce   sync.Once
)

// RunningInUserNS detects whether we are currently running in a user namespace.
// Originally copied from github.com/lxc/lxd/shared/util.go
func RunningInUserNS() bool {
	nsOnce.Do(func() {
		uidmap, err := user.CurrentProcessUIDMap()
		if err != nil {
			// This kernel-provided file only exists if user namespaces are supported
			return
		}
		inUserNS = UIDMapInUserNS(uidmap)
	})
	return inUserNS
}

func UIDMapInUserNS(uidmap []user.IDMap) bool {
=======
/*
 * Detect whether we are currently running in a user namespace.
 * Copied from github.com/lxc/lxd/shared/util.go
 */
func RunningInUserNS() bool {
	file, err := os.Open("/proc/self/uid_map")
	if err != nil {
		/*
		 * This kernel-provided file only exists if user namespaces are
		 * supported
		 */
		return false
	}
	defer file.Close()

	buf := bufio.NewReader(file)
	l, _, err := buf.ReadLine()
	if err != nil {
		return false
	}

	line := string(l)
	var a, b, c int64
	fmt.Sscanf(line, "%d %d %d", &a, &b, &c)
>>>>>>> cbc9bb05... fixup add vendor back
	/*
	 * We assume we are in the initial user namespace if we have a full
	 * range - 4294967295 uids starting at uid 0.
	 */
<<<<<<< HEAD
	if len(uidmap) == 1 && uidmap[0].ID == 0 && uidmap[0].ParentID == 0 && uidmap[0].Count == 4294967295 {
=======
	if a == 0 && b == 0 && c == 4294967295 {
>>>>>>> cbc9bb05... fixup add vendor back
		return false
	}
	return true
}

<<<<<<< HEAD
// GetParentNSeuid returns the euid within the parent user namespace
func GetParentNSeuid() int64 {
	euid := int64(os.Geteuid())
	uidmap, err := user.CurrentProcessUIDMap()
	if err != nil {
		// This kernel-provided file only exists if user namespaces are supported
		return euid
	}
	for _, um := range uidmap {
		if um.ID <= euid && euid <= um.ID+um.Count-1 {
			return um.ParentID + euid - um.ID
		}
	}
	return euid
}

// SetSubreaper sets the value i as the subreaper setting for the calling process
func SetSubreaper(i int) error {
	return unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, uintptr(i), 0, 0, 0)
}

// GetSubreaper returns the subreaper setting for the calling process
func GetSubreaper() (int, error) {
	var i uintptr

	if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&i)), 0, 0, 0); err != nil {
		return -1, err
	}

	return int(i), nil
=======
// SetSubreaper sets the value i as the subreaper setting for the calling process
func SetSubreaper(i int) error {
	return Prctl(PR_SET_CHILD_SUBREAPER, uintptr(i), 0, 0, 0)
}

func Prctl(option int, arg2, arg3, arg4, arg5 uintptr) (err error) {
	_, _, e1 := syscall.Syscall6(syscall.SYS_PRCTL, uintptr(option), arg2, arg3, arg4, arg5, 0)
	if e1 != 0 {
		err = e1
	}
	return
>>>>>>> cbc9bb05... fixup add vendor back
}
