package server

import (
	"net/http"
	"path/filepath"

	"cloudware/cloudware/api"
	"cloudware/cloudware/api/http/server/handler"
	"cloudware/cloudware/api/http/server/security"
	"cloudware/cloudware/api/http/server/proxy"
	"github.com/sirupsen/logrus"
)

// Server implements the cloudware.Server interface
type Server struct {
	BindAddress            string
	AssetsPath             string
	AuthDisabled           bool
	EndpointManagement     bool
	Status                 *api.Status
	UserService            api.UserService
	TeamService            api.TeamService
	TeamMembershipService  api.TeamMembershipService
	EndpointService        api.EndpointService
	ResourceControlService api.ResourceControlService
	SettingsService        api.SettingsService
	CryptoService          api.CryptoService
	JWTService             api.JWTService
	FileService            api.FileService
	RegistryService        api.RegistryService
	DockerHubService       api.DockerHubService
	StackService           api.StackService
	StackManager           api.StackManager
	Handler                *handler.Handler
	SSL                    bool
	SSLCert                string
	SSLKey                 string
}

// Start starts the HTTP server
func (server *Server) Start() error {
	requestBouncer := security.NewRequestBouncer(server.JWTService, server.TeamMembershipService, server.AuthDisabled)
	proxyManager := proxy.NewManager(server.ResourceControlService, server.TeamMembershipService, server.SettingsService)

	var fileHandler = handler.NewFileHandler(filepath.Join(server.AssetsPath, "public"))
	var authHandler = handler.NewAuthHandler(requestBouncer, server.AuthDisabled)
	authHandler.UserService = server.UserService
	authHandler.CryptoService = server.CryptoService
	authHandler.JWTService = server.JWTService
	authHandler.SettingsService = server.SettingsService
	var userHandler = handler.NewUserHandler(requestBouncer)
	userHandler.UserService = server.UserService
	userHandler.TeamService = server.TeamService
	userHandler.TeamMembershipService = server.TeamMembershipService
	userHandler.CryptoService = server.CryptoService
	userHandler.ResourceControlService = server.ResourceControlService
	userHandler.SettingsService = server.SettingsService
	var teamHandler = handler.NewTeamHandler(requestBouncer)
	teamHandler.TeamService = server.TeamService
	teamHandler.TeamMembershipService = server.TeamMembershipService
	var teamMembershipHandler = handler.NewTeamMembershipHandler(requestBouncer)
	teamMembershipHandler.TeamMembershipService = server.TeamMembershipService
	var statusHandler = handler.NewStatusHandler(requestBouncer, server.Status)
	var settingsHandler = handler.NewSettingsHandler(requestBouncer)
	settingsHandler.SettingsService = server.SettingsService
	settingsHandler.FileService = server.FileService
	var templatesHandler = handler.NewTemplatesHandler(requestBouncer)
	templatesHandler.SettingsService = server.SettingsService
	var dockerHandler = handler.NewDockerHandler(requestBouncer)
	dockerHandler.EndpointService = server.EndpointService
	dockerHandler.TeamMembershipService = server.TeamMembershipService
	dockerHandler.ProxyManager = proxyManager
	var websocketHandler = handler.NewWebSocketHandler()
	websocketHandler.EndpointService = server.EndpointService
	var endpointHandler = handler.NewEndpointHandler(requestBouncer, server.EndpointManagement)
	endpointHandler.EndpointService = server.EndpointService
	endpointHandler.FileService = server.FileService
	endpointHandler.ProxyManager = proxyManager
	var registryHandler = handler.NewRegistryHandler(requestBouncer)
	registryHandler.RegistryService = server.RegistryService
	var dockerHubHandler = handler.NewDockerHubHandler(requestBouncer)
	dockerHubHandler.DockerHubService = server.DockerHubService
	var resourceHandler = handler.NewResourceHandler(requestBouncer)
	resourceHandler.ResourceControlService = server.ResourceControlService
	var uploadHandler = handler.NewUploadHandler(requestBouncer)
	uploadHandler.FileService = server.FileService
	var stackHandler = handler.NewStackHandler(requestBouncer)
	stackHandler.FileService = server.FileService
	stackHandler.StackService = server.StackService
	stackHandler.EndpointService = server.EndpointService
	stackHandler.ResourceControlService = server.ResourceControlService
	stackHandler.StackManager = server.StackManager
	stackHandler.RegistryService = server.RegistryService
	stackHandler.DockerHubService = server.DockerHubService

	server.Handler = &handler.Handler{
		AuthHandler:           authHandler,
		UserHandler:           userHandler,
		TeamHandler:           teamHandler,
		TeamMembershipHandler: teamMembershipHandler,
		EndpointHandler:       endpointHandler,
		RegistryHandler:       registryHandler,
		DockerHubHandler:      dockerHubHandler,
		ResourceHandler:       resourceHandler,
		SettingsHandler:       settingsHandler,
		StatusHandler:         statusHandler,
		StackHandler:          stackHandler,
		TemplatesHandler:      templatesHandler,
		DockerHandler:         dockerHandler,
		WebSocketHandler:      websocketHandler,
		FileHandler:           fileHandler,
		UploadHandler:         uploadHandler,
	}

	return nil
}

func (server *Server) Wait(waitChan chan error) {
	var err error
	if server.SSL {
		err = http.ListenAndServeTLS(server.BindAddress, server.SSLCert, server.SSLKey, server.Handler)
	} else {
		err = http.ListenAndServe(server.BindAddress, server.Handler)
	}
	if err != nil {
		logrus.Errorf("ServeAPI error: %v", err)
		waitChan <- err
	}
	waitChan <- nil
}

func (server *Server) Close() error {
	return nil
}