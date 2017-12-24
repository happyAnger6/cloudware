package main

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/sirupsen/logrus"

	"cloudware/cloudware/pkg/pidfile"
	"cloudware/cloudware/api/http/server"
	"cloudware/cloudware/daemon"
	"cloudware/cloudware/daemon/config"
	"cloudware/cloudware/cli/debug"
	"cloudware/cloudware/pkg/signal"
)

type DaemonCli struct {
	*config.Config
	configFile *string
	flags      *pflag.FlagSet
	api        *server.Server
	d          *daemon.Daemon
}

// NewDaemonCli returns a daemon CLI
func NewDaemonCli() *DaemonCli {
	return &DaemonCli{}
}

func (cli *DaemonCli) start(opts *daemonOptions) (err error) {
	stopc := make(chan bool)
	defer close(stopc)

	if cli.Config, err = loadDaemonCliConfig(opts); err != nil {
		return err
	}
	cli.flags = opts.flags

	if cli.Config.Debug {
		debug.Enable()
	}

	if cli.Pidfile != "" {
		pf, err := pidfile.New(cli.Pidfile)
		if err != nil {
			return fmt.Errorf("Error starting daemon: %v", err)
		}
		defer func() {
			if err := pf.Remove(); err != nil {
				logrus.Error(err)
			}
		}()
	}

	api := server.New(opts.httpCliFlags)
	cli.api = api

	signal.Trap(func() {
		cli.stop()
		<-stopc // wait for daemonCli.start() to return
	}, logrus.StandardLogger())

	// Notify that the API is active, but before daemon is set up.
	preNotifySystem()

	// The serve API routine never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	serveAPIWait := make(chan error)
	go api.Wait(serveAPIWait)

	// after the daemon is done setting up we can notify systemd api
	notifySystem()
	// Daemon is fully initialized and handling API traffic
	// Wait for serve API to complete
	errAPI := <-serveAPIWait
	if errAPI != nil {
		return fmt.Errorf("Shutting down due to ServeAPI error: %v", errAPI)
	}

	return nil
}

func (cli *DaemonCli) stop() {
	cli.api.Close()
}

func loadDaemonCliConfig(opts *daemonOptions) (*config.Config, error) {
	conf := opts.daemonConfig
	conf.Debug = opts.Debug
	conf.LogLevel = opts.LogLevel

	// ensure that the log level is the one set after merging configurations
	setLogLevel(conf.LogLevel)

	return conf, nil
}
