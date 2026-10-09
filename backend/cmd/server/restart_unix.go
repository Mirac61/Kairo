//go:build !windows

package main

import (
	"os"
	"syscall"
)

// restartSelf ersetzt den Prozess durch bin (gleiche PID, auch unter launchd und systemd).
func restartSelf(bin string) error { return syscall.Exec(bin, os.Args, os.Environ()) }
