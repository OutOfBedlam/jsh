//go:build linux || darwin

package jsh

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"unsafe"

	"github.com/dop251/goja"
)

func (jr *JSRuntime) exec0(ex *exec.Cmd) goja.Value {
	ex.Stdin = jr.Env.Reader()
	ex.Stdout = jr.Env.Writer()
	ex.Stderr = jr.Env.Writer()

	// Get terminal file descriptor
	ttyFd := int(os.Stdin.Fd())

	// Get shell's process group ID
	shellPgid := syscall.Getpgrp()

	// child process group, false: not separate
	ex.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // new process group
		Pgid:    0,    // use child's PID as pgid
	}

	// Ignore TTY-related signals during job control operations
	signal.Ignore(syscall.SIGTTOU)
	signal.Ignore(syscall.SIGTTIN)
	signal.Ignore(syscall.SIGTSTP)
	defer func() {
		signal.Reset(syscall.SIGTTOU)
		signal.Reset(syscall.SIGTTIN)
		signal.Reset(syscall.SIGTSTP)
	}()

	// child process start
	if err := ex.Start(); err != nil {
		return jr.vm.NewGoError(err)
	}

	childPgid := ex.Process.Pid
	// give terminal control to child process
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(ttyFd),
		syscall.TIOCSPGRP,
		uintptr(unsafe.Pointer(&childPgid)))
	if err != 0 {
		fmt.Printf("failed to set foreground: %v\n", err)
	}

	// wait for process to finish
	var result goja.Value
	if err := ex.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result = jr.vm.ToValue(exitErr.ExitCode())
		} else {
			result = jr.vm.NewGoError(err)
		}
	} else {
		result = jr.vm.ToValue(0)
	}

	// restore this parent process to foreground
	_, _, err = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(os.Stdin.Fd()),
		syscall.TIOCSPGRP,
		uintptr(unsafe.Pointer(&shellPgid)))
	if err != 0 {
		fmt.Printf("failed to restore foreground: %v\n", err)
	} else {
		fmt.Println()
	}
	return result
}
