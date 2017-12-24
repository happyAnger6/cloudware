package cli

import (
	"log"
	"time"

	"cloudware/cloudware/api"

	"os"
	"strings"
	"github.com/spf13/pflag"
)
const (
	defaultBindAddress     = ":9000"
	defaultDataDirectory   = "/data"
	defaultAssetsDirectory = "./"
	defaultNoAuth          = "false"
	defaultNoAnalytics     = "false"
	defaultTLSVerify       = "false"
	defaultTLSCACertPath   = "/certs/ca.pem"
	defaultTLSCertPath     = "/certs/cert.pem"
	defaultTLSKeyPath      = "/certs/key.pem"
	defaultSSL             = "false"
	defaultSSLCertPath     = "/certs/portainer.crt"
	defaultSSLKeyPath      = "/certs/portainer.key"
	defaultSyncInterval    = "60s"
)

func InstallHttpServerFlags(flags *pflag.FlagSet) (*api.HttpCliFlags, error) {
	hCliFlags := &api.HttpCliFlags{}

	flags.StringVar(&hCliFlags.Addr, "bind", defaultBindAddress, "Http address and port to server cloudware.")

	return hCliFlags, nil
}

// Service implements the CLIService interface
type Service struct{}

const (
	errInvalidEndpointProtocol       = api.Error("Invalid endpoint protocol: Cloudware only supports unix:// or tcp://")
	errSocketNotFound                = api.Error("Unable to locate Unix socket")
	errEndpointsFileNotFound         = api.Error("Unable to locate external endpoints file")
	errInvalidSyncInterval           = api.Error("Invalid synchronization interval")
	errEndpointExcludeExternal       = api.Error("Cannot use the -H flag mutually with --external-endpoints")
	errNoAuthExcludeAdminPassword    = api.Error("Cannot use --no-auth with --admin-password or --admin-password-file")
	errAdminPassExcludeAdminPassFile = api.Error("Cannot use --admin-password with --admin-password-file")
)

// ParseFlags parse the CLI flags and return a api.Flags struct
func (*Service) ParseFlags(version string) (*api.HttpCliFlags, error) {
	return nil, nil
}

// ValidateFlags validates the values of the flags.
func (*Service) ValidateFlags(flags *api.HttpCliFlags) error {

	if flags.Endpoint != "" && flags.ExternalEndpoints != "" {
		return errEndpointExcludeExternal
	}

	err := validateEndpoint(flags.Endpoint)
	if err != nil {
		return err
	}

	err = validateExternalEndpoints(flags.ExternalEndpoints)
	if err != nil {
		return err
	}

	err = validateSyncInterval(flags.SyncInterval)
	if err != nil {
		return err
	}

	if flags.NoAuth && (flags.AdminPassword != "" || flags.AdminPasswordFile != "") {
		return errNoAuthExcludeAdminPassword
	}

	if flags.AdminPassword != "" && flags.AdminPasswordFile != "" {
		return errAdminPassExcludeAdminPassFile
	}

	displayDeprecationWarnings(flags.Templates, flags.Logo, *flags.Labels)

	return nil
}

func validateEndpoint(endpoint string) error {
	if endpoint != "" {
		if !strings.HasPrefix(endpoint, "unix://") && !strings.HasPrefix(endpoint, "tcp://") {
			return errInvalidEndpointProtocol
		}

		if strings.HasPrefix(endpoint, "unix://") {
			socketPath := strings.TrimPrefix(endpoint, "unix://")
			if _, err := os.Stat(socketPath); err != nil {
				if os.IsNotExist(err) {
					return errSocketNotFound
				}
				return err
			}
		}
	}
	return nil
}

func validateExternalEndpoints(externalEndpoints string) error {
	if externalEndpoints != "" {
		if _, err := os.Stat(externalEndpoints); err != nil {
			if os.IsNotExist(err) {
				return errEndpointsFileNotFound
			}
			return err
		}
	}
	return nil
}

func validateSyncInterval(syncInterval string) error {
	if syncInterval != defaultSyncInterval {
		_, err := time.ParseDuration(syncInterval)
		if err != nil {
			return errInvalidSyncInterval
		}
	}
	return nil
}

func displayDeprecationWarnings(templates, logo string, labels []api.Pair) {
	if templates != "" {
		log.Println("Warning: the --templates / -t flag is deprecated and will be removed in future versions.")
	}
	if logo != "" {
		log.Println("Warning: the --logo flag is deprecated and will be removed in future versions.")
	}
	if labels != nil {
		log.Println("Warning: the --hide-label / -l flag is deprecated and will be removed in future versions.")
	}
}
