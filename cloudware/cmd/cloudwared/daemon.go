package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"github.com/sirupsen/logrus"

	"cloudware/cloudware/pkg/pidfile"
	"cloudware/cloudware/api"
	"cloudware/cloudware/daemon"
	"cloudware/cloudware/daemon/config"
	"cloudware/cloudware/cli/debug"
	"os/signal"
)

type DaemonCli struct {
	*config.Config
	configFile *string
	flags      *pflag.FlagSet
	api        *api.Server
	d          *daemon.Daemon
}

// NewDaemonCli returns a daemon CLI
func NewDaemonCli() *DaemonCli {
	return &DaemonCli{}
}

func (cli *DaemonCli) start(opts daemonOptions) (err error) {
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

	serverConfig := &apiserver.Config{
		Logging:     true,
		SocketGroup: cli.Config.SocketGroup,
		Version:     dockerversion.Version,
		EnableCors:  cli.Config.EnableCors,
		CorsHeaders: cli.Config.CorsHeaders,
	}

	api := apiserver.New(serverConfig)
	cli.api = api

	signal.Trap(func() {
		cli.stop()
		<-stopc // wait for daemonCli.start() to return
	})

	// Notify that the API is active, but before daemon is set up.
	preNotifySystem()

	d, err := daemon.NewDaemon(cli.Config, registryService, containerdRemote, pluginStore)
	if err != nil {
		return fmt.Errorf("Error starting daemon: %v", err)
	}

	name, _ := os.Hostname()

	logrus.Info("Daemon has completed initialization")

	logrus.WithFields(logrus.Fields{
		"version":     dockerversion.Version,
		"commit":      dockerversion.GitCommit,
		"graphdriver": d.GraphDriverName(),
	}).Info("Docker daemon")

	cli.d = d

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
	c.Cleanup()
	shutdownDaemon(d)
	if errAPI != nil {
		return fmt.Errorf("Shutting down due to ServeAPI error: %v", errAPI)
	}

	return nil
}

func (cli *DaemonCli) reloadConfig() {
	reload := func(config *config.Config) {

		// Revalidate and reload the authorization plugins
		if err := validateAuthzPlugins(config.AuthorizationPlugins, cli.d.PluginStore); err != nil {
			logrus.Fatalf("Error validating authorization plugin: %v", err)
			return
		}
		cli.authzMiddleware.SetPlugins(config.AuthorizationPlugins)

		if err := cli.d.Reload(config); err != nil {
			logrus.Errorf("Error reconfiguring the daemon: %v", err)
			return
		}

		if config.IsValueSet("debug") {
			debugEnabled := debug.IsEnabled()
			switch {
			case debugEnabled && !config.Debug: // disable debug
				debug.Disable()
				cli.api.DisableProfiler()
			case config.Debug && !debugEnabled: // enable debug
				debug.Enable()
				cli.api.EnableProfiler()
			}

		}
	}

	if err := config.Reload(*cli.configFile, cli.flags, reload); err != nil {
		logrus.Error(err)
	}
}

func (cli *DaemonCli) stop() {
	cli.api.Close()
}

// shutdownDaemon just wraps daemon.Shutdown() to handle a timeout in case
// d.Shutdown() is waiting too long to kill container or worst it's
// blocked there
func shutdownDaemon(d *daemon.Daemon) {
	shutdownTimeout := d.ShutdownTimeout()
	ch := make(chan struct{})
	go func() {
		d.Shutdown()
		close(ch)
	}()
	if shutdownTimeout < 0 {
		<-ch
		logrus.Debug("Clean shutdown succeeded")
		return
	}
	select {
	case <-ch:
		logrus.Debug("Clean shutdown succeeded")
	case <-time.After(time.Duration(shutdownTimeout) * time.Second):
		logrus.Error("Force shutdown daemon")
	}
}

func loadDaemonCliConfig(opts daemonOptions) (*config.Config, error) {
	conf := opts.daemonConfig
	flags := opts.flags
	conf.Debug = opts.common.Debug
	conf.Hosts = opts.common.Hosts
	conf.LogLevel = opts.common.LogLevel
	conf.TLS = opts.common.TLS
	conf.TLSVerify = opts.common.TLSVerify
	conf.CommonTLSOptions = config.CommonTLSOptions{}

	if opts.common.TLSOptions != nil {
		conf.CommonTLSOptions.CAFile = opts.common.TLSOptions.CAFile
		conf.CommonTLSOptions.CertFile = opts.common.TLSOptions.CertFile
		conf.CommonTLSOptions.KeyFile = opts.common.TLSOptions.KeyFile
	}

	if flags.Changed("graph") && flags.Changed("data-root") {
		return nil, fmt.Errorf(`cannot specify both "--graph" and "--data-root" option`)
	}

	if opts.configFile != "" {
		c, err := config.MergeDaemonConfigurations(conf, flags, opts.configFile)
		if err != nil {
			if flags.Changed("config-file") || !os.IsNotExist(err) {
				return nil, fmt.Errorf("unable to configure the Docker daemon with file %s: %v\n", opts.configFile, err)
			}
		}
		// the merged configuration can be nil if the config file didn't exist.
		// leave the current configuration as it is if when that happens.
		if c != nil {
			conf = c
		}
	}

	if err := config.Validate(conf); err != nil {
		return nil, err
	}

	if flags.Changed("graph") {
		logrus.Warnf(`the "-g / --graph" flag is deprecated. Please use "--data-root" instead`)
	}

	// Labels of the docker engine used to allow multiple values associated with the same key.
	// This is deprecated in 1.13, and, be removed after 3 release cycles.
	// The following will check the conflict of labels, and report a warning for deprecation.
	//
	// TODO: After 3 release cycles (17.12) an error will be returned, and labels will be
	// sanitized to consolidate duplicate key-value pairs (config.Labels = newLabels):
	//
	// newLabels, err := daemon.GetConflictFreeLabels(config.Labels)
	// if err != nil {
	//	return nil, err
	// }
	// config.Labels = newLabels
	//
	if _, err := config.GetConflictFreeLabels(conf.Labels); err != nil {
		logrus.Warnf("Engine labels with duplicate keys and conflicting values have been deprecated: %s", err)
	}

	// Regardless of whether the user sets it to true or false, if they
	// specify TLSVerify at all then we need to turn on TLS
	if conf.IsValueSet(cliflags.FlagTLSVerify) {
		conf.TLS = true
	}

	// ensure that the log level is the one set after merging configurations
	cliflags.SetLogLevel(conf.LogLevel)

	return conf, nil
}
