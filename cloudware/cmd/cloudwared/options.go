package main

import (
	"os"
	"fmt"
	"github.com/spf13/pflag"
	"github.com/sirupsen/logrus"
)

type cloudwareOptions struct {
	version bool
	LogLevel string
	RunMode string
	Debug bool
}

// newDaemonOptions returns a new daemonFlags
func newCloudwareOptions() *cloudwareOptions {
	return &cloudwareOptions{
	}
}

// InstallFlags adds flags for the common options on the FlagSet
func (o *cloudwareOptions) InstallFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&o.Debug, "debug", "D", false, "Enable debug mode")
	flags.StringVarP(&o.LogLevel, "log-level", "l", "info", `Set the logging level ("debug"|"info"|"warn"|"error"|"fatal")`)
}

// SetDefaultOptions sets default values for options after flag parsing is
// complete
func (o *cloudwareOptions) SetDefaultOptions(flags *pflag.FlagSet) {
}

// setLogLevel sets the logrus logging level
func setLogLevel(logLevel string) {
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to parse logging level: %s\n", logLevel)
			os.Exit(1)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}