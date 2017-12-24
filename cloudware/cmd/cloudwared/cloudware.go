package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"cloudware/cloudware/cli"
	"cloudware/cloudware/pkg/term"
	"cloudware/cloudware/daemon/config"
	apiCli "cloudware/cloudware/api/cli"
)



const (
	version="0.1"
)

func newDaemonCommand() *cobra.Command {
	opts := newDaemonOptions(config.New())

	cmd := &cobra.Command{
		Use:           "cloudward [OPTIONS]",
		Short:         "A management platform for containers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.flags = cmd.Flags()
			return runDaemon(opts)
		},
	}
	cli.SetupRootCommand(cmd)

	flags := cmd.Flags()
	flags.BoolVarP(&opts.version, "version", "v", false, "Print version information and quit")

	opts.httpCliFlags, _ = apiCli.InstallHttpServerFlags(flags);
	return cmd
}

func runDaemon(opts *daemonOptions) error {
	if opts.version {
		showVersion()
		return nil
	}

	daemonCli := NewDaemonCli()

	// Windows specific settings as these are not defaulted.
	if runtime.GOOS == "windows" {
		if opts.daemonConfig.Pidfile == "" {
			opts.daemonConfig.Pidfile = filepath.Join(opts.daemonConfig.Root, "cloudware.pid")
		}
		if opts.configFile == "" {
			opts.configFile = filepath.Join(opts.daemonConfig.Root, `config\daemon.json`)
		}
	}

	err := daemonCli.start(opts)
	notifyShutdown(err)
	return err
}

func showVersion() {
	fmt.Printf("Cloudware version %s\n", version)
}

func main() {
	// Set terminal emulation based on platform as required.
	_, stdout, stderr := term.StdStreams()

	// @jhowardmsft - maybe there is a historic reason why on non-Windows, stderr is used
	// here. However, on Windows it makes no sense and there is no need.
	if runtime.GOOS == "windows" {
		logrus.SetOutput(stdout)
	} else {
		logrus.SetOutput(stderr)
	}

	cmd := newDaemonCommand()
	cmd.SetOutput(stdout)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		os.Exit(1)
	}
}
