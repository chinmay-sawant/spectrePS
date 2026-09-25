package cli

import (
	"fmt"
	"io"
)

// gsCommand is one parsed `spectreps gs` command line. The scanner fills the
// fields, and argv builds the command line for the existing subcommand.
type gsCommand struct {
	command string
	input   string
}

// cmdGS runs the bounded gs argv mode. Only the switches listed in
// documentation/gs-argv-grammar.md are accepted, and the job routes to an
// existing subcommand.
func cmdGS(args []string, stdout, stderr io.Writer) int {
	job, code := scanGS(args, stderr)
	if code != exitOK {
		return code
	}
	return dispatch(job.argv(), stdout, stderr)
}

// scanGS parses one gs argv. The device and output families land next, so no
// switch sets a command yet.
func scanGS(args []string, stderr io.Writer) (gsCommand, int) {
	job := gsCommand{command: "", input: ""}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-f" && i+1 < len(args) {
			i++
			job.input = args[i]
			continue
		}
		return gsCommand{command: "", input: ""}, gsError(stderr, "%s is not in the gs allowlist", arg)
	}
	if job.input == "" {
		usage(stderr)
		return gsCommand{command: "", input: ""}, exitUsage
	}
	return job, exitOK
}

// argv builds the command line for the routed subcommand.
func (job gsCommand) argv() []string {
	return []string{job.command}
}

// gsError writes a rejection that names the switch and returns the usage exit
// code.
func gsError(stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, "spectreps: "+format+"\n", args...)
	return exitUsage
}
