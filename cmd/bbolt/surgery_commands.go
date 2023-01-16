package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"go.etcd.io/bbolt/internal/surgeon"
)

// SurgeryCommand represents the "surgery" command execution.
type SurgeryCommand struct {
	baseCommand
}

// newSurgeryCommand returns a SurgeryCommand.
func newSurgeryCommand(m *Main) *SurgeryCommand {
	c := &SurgeryCommand{}
	c.baseCommand = m.baseCommand
	return c
}

// Run executes the `surgery` program.
func (cmd *SurgeryCommand) Run(args ...string) error {
	// Require a command at the beginning.
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(cmd.Stderr, cmd.Usage())
		return ErrUsage
	}

	// Execute command.
	switch args[0] {
	case "help":
		fmt.Fprintln(cmd.Stderr, cmd.Usage())
		return ErrUsage
	case "revert-meta-page":
		return newRevertMetaPageCommand(cmd).Run(args[1:]...)
	default:
		return ErrUnknownCommand
	}
}

// Usage returns the help message.
func (cmd *SurgeryCommand) Usage() string {
	return strings.TrimLeft(`
Surgery is a command for performing low level update on bbolt databases.

Usage:

	bbolt surgery command [arguments]

The commands are:

    help                   print this screen
    revert-meta-page       revert the meta page change made by the last transaction

Use "bbolt surgery [command] -h" for more information about a command.
`, "\n")
}

// RevertMetaPageCommand represents the "surgery revert-meta-page" command execution.
type RevertMetaPageCommand struct {
	baseCommand

	SrcPath string
	DstPath string
}

// newRevertMetaPageCommand returns a RevertMetaPageCommand.
func newRevertMetaPageCommand(m *SurgeryCommand) *RevertMetaPageCommand {
	c := &RevertMetaPageCommand{}
	c.baseCommand = m.baseCommand
	return c
}

// Run executes the command.
func (cmd *RevertMetaPageCommand) Run(args ...string) error {
	// Parse flags.
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cmd.DstPath, "o", "", "")
	if err := fs.Parse(args); err == flag.ErrHelp {
		fmt.Fprintln(cmd.Stderr, cmd.Usage())
		return ErrUsage
	} else if err != nil {
		return err
	} else if cmd.DstPath == "" {
		return errors.New("output file required")
	}

	// Require database paths.
	cmd.SrcPath = fs.Arg(0)
	if cmd.SrcPath == "" {
		return ErrPathRequired
	}

	// Ensure source file exists.
	fi, err := os.Stat(cmd.SrcPath)
	if os.IsNotExist(err) {
		return ErrFileNotFound
	} else if err != nil {
		return err
	}
	initialSize := fi.Size()

	// Ensure output file not exist.
	_, err = os.Stat(cmd.DstPath)
	if err == nil {
		return fmt.Errorf("output file %q already exists", cmd.DstPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	// Copy database from SrcPath to DstPath
	srcDB, err := os.Open(cmd.SrcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", cmd.SrcPath, err)
	}
	defer srcDB.Close()
	dstDB, err := os.Create(cmd.DstPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %w", cmd.DstPath, err)
	}
	defer dstDB.Close()
	written, err := io.Copy(dstDB, srcDB)
	if err != nil {
		return fmt.Errorf("failed to copy database file from %q to %q: %w", cmd.SrcPath, cmd.DstPath, err)
	}
	if initialSize != written {
		return fmt.Errorf("the byte copied (%q: %d) isn't equal to the initial db size (%q: %d)", cmd.DstPath, written, cmd.SrcPath, initialSize)
	}

	// revert the meta page
	if err = surgeon.RevertMetaPage(cmd.DstPath); err != nil {
		return err
	}

	fmt.Fprintln(cmd.Stdout, "The meta page is reverted.")
	return nil
}

// Usage returns the help message.
func (cmd *RevertMetaPageCommand) Usage() string {
	return strings.TrimLeft(`
usage: bolt surgery revert-meta-page -o DST SRC

RevertMetaPage copies the database file at SRC to a newly created database
file at DST. Afterwards, it reverts the meta page on the newly created
database at DST.

The original database is left untouched.
`, "\n")
}
