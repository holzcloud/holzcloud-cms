// Command hc-canary measures what a sandboxed process is allowed to do.
//
// deploy/test/run.sh starts it twice under the sandbox properties of
// deploy/holzcloud.service (systemd-run), once with MemoryDenyWriteExecute=yes
// and once with =no as a control. The difference between the two runs, and the
// Seccomp lines at the end, say whether the platform enforces the seccomp-based
// directives at all. Under Rosetta it does not, and a green plugin run there
// would prove nothing.
//
// The probe prints one line per test and always exits 0: a denial is the
// measurement, the verdict belongs to run.sh. Standard library only, so the
// repository gains no module dependency.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
)

// labelWidth is the column the ALLOWED/denied text starts in.
const labelWidth = 22

// afNetlink is AF_NETLINK on Linux. Package syscall defines the name only for
// linux, and the literal keeps `go vet ./...` working on a developer's Mac; the
// probe is only ever run in the Linux container.
const afNetlink = 16

func main() {
	report("mmap RW", probeMmapRW())
	report("mprotect RW->RX", probeMprotectRX())
	report("mmap RWX", probeMmapRWX())
	report("socket AF_NETLINK", probeSocket(afNetlink, syscall.SOCK_RAW))
	report("socket AF_INET", probeSocket(syscall.AF_INET, syscall.SOCK_STREAM))
	report("stat /proc/sys", probeStat("/proc/sys"))
	report("stat /dev/kmsg", probeStat("/dev/kmsg"))
	report("write /etc", probeWrite("/etc/hc-canary-probe"))
	printSeccomp()
}

// report prints the label padded to labelWidth, then ALLOWED or the denial.
func report(label string, err error) {
	result := "ALLOWED"
	if err != nil {
		result = "denied (" + err.Error() + ")"
	}
	fmt.Printf("%-*s %s\n", labelWidth, label, result)
}

// anonymous maps one private page of anonymous memory with the given protection.
func anonymous(prot int) ([]byte, error) {
	return syscall.Mmap(-1, 0, os.Getpagesize(), prot, syscall.MAP_ANON|syscall.MAP_PRIVATE)
}

func probeMmapRW() error {
	b, err := anonymous(syscall.PROT_READ | syscall.PROT_WRITE)
	if err != nil {
		return err
	}
	return syscall.Munmap(b)
}

// probeMprotectRX is the step wazero's own capability probe takes: map RW, then
// flip the mapping to RX. MemoryDenyWriteExecute refuses the second call.
func probeMprotectRX() error {
	b, err := anonymous(syscall.PROT_READ | syscall.PROT_WRITE)
	if err != nil {
		return err
	}
	defer func() { _ = syscall.Munmap(b) }()
	return syscall.Mprotect(b, syscall.PROT_READ|syscall.PROT_EXEC)
}

// probeMmapRWX asks for writable and executable at once.
func probeMmapRWX() error {
	b, err := anonymous(syscall.PROT_READ | syscall.PROT_WRITE | syscall.PROT_EXEC)
	if err != nil {
		return err
	}
	return syscall.Munmap(b)
}

func probeSocket(domain, typ int) error {
	fd, err := syscall.Socket(domain, typ, 0)
	if err != nil {
		return err
	}
	return syscall.Close(fd)
}

func probeStat(path string) error {
	_, err := os.Stat(path)
	return unwrap(err)
}

// probeWrite tries to create a file; ProtectSystem=strict makes /etc read-only.
func probeWrite(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return unwrap(err)
	}
	_ = f.Close()
	return os.Remove(path)
}

// unwrap reduces an *os.PathError to the bare errno text, so the report reads
// "read-only file system" rather than repeating the path.
func unwrap(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// printSeccomp prints the Seccomp and Seccomp_filters lines of
// /proc/self/status verbatim. Seccomp: 0 means no filter is installed.
func printSeccomp() {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		fmt.Printf("%-*s denied (%v)\n", labelWidth, "read /proc/self/status", unwrap(err))
		return
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "Seccomp") {
			fmt.Println(sc.Text())
		}
	}
}
