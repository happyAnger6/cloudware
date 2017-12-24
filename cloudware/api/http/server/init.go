package server

import (
	"log"

	"cloudware/cloudware/api"
	"cloudware/cloudware/api/file"
	"cloudware/cloudware/bolt"
	"cloudware/cloudware/api/http/server/jwt"
	"cloudware/cloudware/api/exec"
	"cloudware/cloudware/api/crypto"
)

func initFileService(dataStorePath string) api.FileService {
	fileService, err := file.NewService(dataStorePath, "")
	if err != nil {
		log.Fatal(err)
	}
	return fileService
}

func initStore(dataStorePath string) *bolt.Store {
	store, err := bolt.NewStore(dataStorePath)
	if err != nil {
		log.Fatal(err)
	}

	err = store.Open()
	if err != nil {
		log.Fatal(err)
	}

	err = store.MigrateData()
	if err != nil {
		log.Fatal(err)
	}
	return store
}

func initStackManager(assetsPath string) api.StackManager {
	return exec.NewStackManager(assetsPath)
}

func initJWTService(authenticationEnabled bool) api.JWTService {
	if authenticationEnabled {
		jwtService, err := jwt.NewService()
		if err != nil {
			log.Fatal(err)
		}
		return jwtService
	}
	return nil
}

func initCryptoService() api.CryptoService {
	return &crypto.Service{}
}

func initLDAPService() api.LDAPService {
	return &ldap.Service{}
}

func initGitService() api.GitService {
	return &git.Service{}
}

func initEndpointWatcher(endpointService api.EndpointService, externalEnpointFile string, syncInterval string) bool {
	authorizeEndpointMgmt := true
	if externalEnpointFile != "" {
		authorizeEndpointMgmt = false
		log.Println("Using external endpoint definition. Endpoint management via the API will be disabled.")
		endpointWatcher := cron.NewWatcher(endpointService, syncInterval)
		err := endpointWatcher.WatchEndpointFile(externalEnpointFile)
		if err != nil {
			log.Fatal(err)
		}
	}
	return authorizeEndpointMgmt
}

func initStatus(authorizeEndpointMgmt bool, flags *api.CLIFlags) *api.Status {
	return &api.Status{
		Analytics:          !*flags.NoAnalytics,
		Authentication:     !*flags.NoAuth,
		EndpointManagement: authorizeEndpointMgmt,
		Version:            api.APIVersion,
	}
}

func initDockerHub(dockerHubService api.DockerHubService) error {
	_, err := dockerHubService.DockerHub()
	if err == api.ErrDockerHubNotFound {
		dockerhub := &api.DockerHub{
			Authentication: false,
			Username:       "",
			Password:       "",
		}
		return dockerHubService.StoreDockerHub(dockerhub)
	} else if err != nil {
		return err
	}

	return nil
}

func initSettings(settingsService api.SettingsService, flags *api.CLIFlags) error {
	_, err := settingsService.Settings()
	if err == api.ErrSettingsNotFound {
		settings := &api.Settings{
			LogoURL:                     *flags.Logo,
			DisplayDonationHeader:       true,
			DisplayExternalContributors: false,
			AuthenticationMethod:        api.AuthenticationInternal,
			LDAPSettings: api.LDAPSettings{
				TLSConfig: api.TLSConfiguration{},
				SearchSettings: []api.LDAPSearchSettings{
					api.LDAPSearchSettings{},
				},
			},
			AllowBindMountsForRegularUsers:     true,
			AllowPrivilegedModeForRegularUsers: true,
		}

		if *flags.Templates != "" {
			settings.TemplatesURL = *flags.Templates
		} else {
			settings.TemplatesURL = api.DefaultTemplatesURL
		}

		if *flags.Labels != nil {
			settings.BlackListedLabels = *flags.Labels
		} else {
			settings.BlackListedLabels = make([]api.Pair, 0)
		}

		return settingsService.StoreSettings(settings)
	} else if err != nil {
		return err
	}

	return nil
}

func retrieveFirstEndpointFromDatabase(endpointService api.EndpointService) *api.Endpoint {
	endpoints, err := endpointService.Endpoints()
	if err != nil {
		log.Fatal(err)
	}
	return &endpoints[0]
}

func New() *api.Server{
	flags := initCLI()

	fileService := initFileService(*flags.Data)

	store := initStore(*flags.Data)
	defer store.Close()

	stackManager := initStackManager(*flags.Assets)

	jwtService := initJWTService(!*flags.NoAuth)

	cryptoService := initCryptoService()

	ldapService := initLDAPService()

	gitService := initGitService()

	authorizeEndpointMgmt := initEndpointWatcher(store.EndpointService, *flags.ExternalEndpoints, *flags.SyncInterval)

	err := initSettings(store.SettingsService, flags)
	if err != nil {
		log.Fatal(err)
	}

	err = initDockerHub(store.DockerHubService)
	if err != nil {
		log.Fatal(err)
	}

	applicationStatus := initStatus(authorizeEndpointMgmt, flags)

	if *flags.Endpoint != "" {
		endpoints, err := store.EndpointService.Endpoints()
		if err != nil {
			log.Fatal(err)
		}
		if len(endpoints) == 0 {
			endpoint := &api.Endpoint{
				Name: "primary",
				URL:  *flags.Endpoint,
				TLSConfig: api.TLSConfiguration{
					TLS:           *flags.TLSVerify,
					TLSSkipVerify: false,
					TLSCACertPath: *flags.TLSCacert,
					TLSCertPath:   *flags.TLSCert,
					TLSKeyPath:    *flags.TLSKey,
				},
				AuthorizedUsers: []api.UserID{},
				AuthorizedTeams: []api.TeamID{},
			}
			err = store.EndpointService.CreateEndpoint(endpoint)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Println("Instance already has defined endpoints. Skipping the endpoint defined via CLI.")
		}
	}

	adminPasswordHash := ""
	if *flags.AdminPasswordFile != "" {
		content, err := fileService.GetFileContent(*flags.AdminPasswordFile)
		if err != nil {
			log.Fatal(err)
		}
		adminPasswordHash, err = cryptoService.Hash(content)
		if err != nil {
			log.Fatal(err)
		}
	} else if *flags.AdminPassword != "" {
		adminPasswordHash = *flags.AdminPassword
	}

	if adminPasswordHash != "" {
		users, err := store.UserService.UsersByRole(api.AdministratorRole)
		if err != nil {
			log.Fatal(err)
		}

		if len(users) == 0 {
			log.Printf("Creating admin user with password hash %s", adminPasswordHash)
			user := &api.User{
				Username: "admin",
				Role:     api.AdministratorRole,
				Password: adminPasswordHash,
			}
			err := store.UserService.CreateUser(user)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Println("Instance already has an administrator user defined. Skipping admin password related flags.")
		}
	}

	var server api.Server = &Server{
		Status:                 applicationStatus,
		BindAddress:            *flags.Addr,
		AssetsPath:             *flags.Assets,
		AuthDisabled:           *flags.NoAuth,
		EndpointManagement:     authorizeEndpointMgmt,
		UserService:            store.UserService,
		TeamService:            store.TeamService,
		TeamMembershipService:  store.TeamMembershipService,
		EndpointService:        store.EndpointService,
		ResourceControlService: store.ResourceControlService,
		SettingsService:        store.SettingsService,
		RegistryService:        store.RegistryService,
		DockerHubService:       store.DockerHubService,
		StackService:           store.StackService,
		StackManager:           stackManager,
		CryptoService:          cryptoService,
		JWTService:             jwtService,
		FileService:            fileService,
		LDAPService:            ldapService,
		GitService:             gitService,
		SSL:                    *flags.SSL,
		SSLCert:                *flags.SSLCert,
		SSLKey:                 *flags.SSLKey,
	}

	log.Printf("Starting Portainer %s on %s", api.APIVersion, *flags.Addr)
	err = server.Start()
	if err != nil {
		log.Fatal(err)
	}

	return server
}
