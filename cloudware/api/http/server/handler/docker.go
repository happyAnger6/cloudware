package handler

import (
	"strconv"

	httperror "cloudware/cloudware/api/http/server/error"
	"cloudware/cloudware/api/http/server/proxy"
	"cloudware/cloudware/api/http/server/security"
	"cloudware/cloudware/api"

	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// DockerHandler represents an HTTP API handler for proxying requests to the Docker API.
type DockerHandler struct {
	*mux.Router
	Logger                *log.Logger
	EndpointService       api.EndpointService
	TeamMembershipService api.TeamMembershipService
	ProxyManager          *proxy.Manager
}

// NewDockerHandler returns a new instance of DockerHandler.
func NewDockerHandler(bouncer *security.RequestBouncer) *DockerHandler {
	h := &DockerHandler{
		Router: mux.NewRouter(),
		Logger: log.New(os.Stderr, "", log.LstdFlags),
	}
	h.PathPrefix("/{id}/docker").Handler(
		bouncer.AuthenticatedAccess(http.HandlerFunc(h.proxyRequestsToDockerAPI)))
	return h
}

func (handler *DockerHandler) checkEndpointAccessControl(endpoint *api.Endpoint, userID api.UserID) bool {
	for _, authorizedUserID := range endpoint.AuthorizedUsers {
		if authorizedUserID == userID {
			return true
		}
	}

	memberships, _ := handler.TeamMembershipService.TeamMembershipsByUserID(userID)
	for _, authorizedTeamID := range endpoint.AuthorizedTeams {
		for _, membership := range memberships {
			if membership.TeamID == authorizedTeamID {
				return true
			}
		}
	}
	return false
}

func (handler *DockerHandler) proxyRequestsToDockerAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	parsedID, err := strconv.Atoi(id)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	endpointID := api.EndpointID(parsedID)
	endpoint, err := handler.EndpointService.Endpoint(endpointID)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	tokenData, err := security.RetrieveTokenData(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
	if tokenData.Role != api.AdministratorRole && !handler.checkEndpointAccessControl(endpoint, tokenData.ID) {
		httperror.WriteErrorResponse(w, api.ErrEndpointAccessDenied, http.StatusForbidden, handler.Logger)
		return
	}

	var proxy http.Handler
	proxy = handler.ProxyManager.GetProxy(string(endpointID))
	if proxy == nil {
		proxy, err = handler.ProxyManager.CreateAndRegisterProxy(endpoint)
		if err != nil {
			httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
			return
		}
	}

	http.StripPrefix("/"+id+"/docker", proxy).ServeHTTP(w, r)
}
