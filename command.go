// Convenience functions for running other commands.

package creeperkeeper

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func runCmd(cmd *exec.Cmd) (stdout []byte, err error) {
	cmdLine := strings.Join(cmd.Args, " ")
	if Verbose {
		log.Println("run:", cmdLine)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout, err = cmd.Output()
	if err == nil && len(stderr.String()) > 0 {
		log.Print("exit status 0: ", cmdLine, "\nstderr:\n", stderr.String())
	}
	if err != nil {
		return stdout, fmt.Errorf("%s: %s\nstderr:\n%s", err.Error(), cmdLine, stderr.String())
	}
	return stdout, nil
}
