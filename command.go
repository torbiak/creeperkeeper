// Convenience functions for running other commands.

package creeperkeeper

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func runCmd(cmd *exec.Cmd) error {
	cmdLine := strings.Join(cmd.Args, " ")
	if Verbose {
		log.Println("run:", cmdLine)
	}
	b := &bytes.Buffer{}
	cmd.Stderr = b
	err := cmd.Run()
	if err == nil && len(b.String()) > 0 {
		log.Print("exit status 0: ", cmdLine, "\nstderr:\n", b.String())
	}
	if err != nil {
		return fmt.Errorf("%s: %s\nstderr:\n%s", err.Error(), cmdLine, b.String())
	}
	return nil
}
