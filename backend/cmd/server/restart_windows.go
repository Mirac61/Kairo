package main

import (
	"os"
	"os/exec"
)

// restartSelf startet bin als neuen Prozess (Windows kennt kein exec); dieser
// beendet sich danach. Ausgabe und Log-Umleitung erbt der neue Prozess.
func restartSelf(bin string) error {
	cmd := exec.Command(bin, os.Args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Start()
}
