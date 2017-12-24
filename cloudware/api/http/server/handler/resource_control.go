package handler

import (
	"encoding/json"
	"strconv"

	"github.com/asaskevich/govalidator"
	"cloudware/cloudware/api"
	httperror "cloudware/cloudware/api/http/server/error"
	"cloudware/cloudware/api/http/server/security"

	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// ResourceHandler represents an HTTP API handler for managing resource controls.
type ResourceHandler struct {
	*mux.Router
	Logger                 *log.Logger
	ResourceControlService api.ResourceControlService
}

// NewResourceHandler returns a new instance of ResourceHandler.
func NewResourceHandler(bouncer *security.RequestBouncer) *ResourceHandler {
	h := &ResourceHandler{
		Router: mux.NewRouter(),
		Logger: log.New(os.Stderr, "", log.LstdFlags),
	}
	h.Handle("/resource_controls",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handlePostResources))).Methods(http.MethodPost)
	h.Handle("/resource_controls/{id}",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handlePutResources))).Methods(http.MethodPut)
	h.Handle("/resource_controls/{id}",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handleDeleteResources))).Methods(http.MethodDelete)

	return h
}

type (
	postResourcesRequest struct {
		ResourceID         string   `valid:"required"`
		Type               string   `valid:"required"`
		AdministratorsOnly bool     `valid:"-"`
		Users              []int    `valid:"-"`
		Teams              []int    `valid:"-"`
		SubResourceIDs     []string `valid:"-"`
	}

	putResourcesRequest struct {
		AdministratorsOnly bool  `valid:"-"`
		Users              []int `valid:"-"`
		Teams              []int `valid:"-"`
	}
)

// handlePostResources handles POST requests on /resources
func (handler *ResourceHandler) handlePostResources(w http.ResponseWriter, r *http.Request) {
	var req postResourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidJSON, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err := govalidator.ValidateStruct(req)
	if err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	var resourceControlType api.ResourceControlType
	switch req.Type {
	case "container":
		resourceControlType = api.ContainerResourceControl
	case "service":
		resourceControlType = api.ServiceResourceControl
	case "volume":
		resourceControlType = api.VolumeResourceControl
	case "network":
		resourceControlType = api.NetworkResourceControl
	case "secret":
		resourceControlType = api.SecretResourceControl
	case "stack":
		resourceControlType = api.StackResourceControl
	case "config":
		resourceControlType = api.ConfigResourceControl
	default:
		httperror.WriteErrorResponse(w, api.ErrInvalidResourceControlType, http.StatusBadRequest, handler.Logger)
		return
	}

	if len(req.Users) == 0 && len(req.Teams) == 0 && !req.AdministratorsOnly {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	rc, err := handler.ResourceControlService.ResourceControlByResourceID(req.ResourceID)
	if err != nil && err != api.ErrResourceControlNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
	if rc != nil {
		httperror.WriteErrorResponse(w, api.ErrResourceControlAlreadyExists, http.StatusConflict, handler.Logger)
		return
	}

	var userAccesses = make([]api.UserResourceAccess, 0)
	for _, v := range req.Users {
		userAccess := api.UserResourceAccess{
			UserID:      api.UserID(v),
			AccessLevel: api.ReadWriteAccessLevel,
		}
		userAccesses = append(userAccesses, userAccess)
	}

	var teamAccesses = make([]api.TeamResourceAccess, 0)
	for _, v := range req.Teams {
		teamAccess := api.TeamResourceAccess{
			TeamID:      api.TeamID(v),
			AccessLevel: api.ReadWriteAccessLevel,
		}
		teamAccesses = append(teamAccesses, teamAccess)
	}

	resourceControl := api.ResourceControl{
		ResourceID:         req.ResourceID,
		SubResourceIDs:     req.SubResourceIDs,
		Type:               resourceControlType,
		AdministratorsOnly: req.AdministratorsOnly,
		UserAccesses:       userAccesses,
		TeamAccesses:       teamAccesses,
	}

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	if !security.AuthorizedResourceControlCreation(&resourceControl, securityContext) {
		httperror.WriteErrorResponse(w, api.ErrResourceAccessDenied, http.StatusForbidden, handler.Logger)
		return
	}

	err = handler.ResourceControlService.CreateResourceControl(&resourceControl)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	return
}

// handlePutResources handles PUT requests on /resources/:id
func (handler *ResourceHandler) handlePutResources(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	resourceControlID, err := strconv.Atoi(id)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	var req putResourcesRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidJSON, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err = govalidator.ValidateStruct(req)
	if err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	resourceControl, err := handler.ResourceControlService.ResourceControl(api.ResourceControlID(resourceControlID))

	if err == api.ErrResourceControlNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	resourceControl.AdministratorsOnly = req.AdministratorsOnly

	var userAccesses = make([]api.UserResourceAccess, 0)
	for _, v := range req.Users {
		userAccess := api.UserResourceAccess{
			UserID:      api.UserID(v),
			AccessLevel: api.ReadWriteAccessLevel,
		}
		userAccesses = append(userAccesses, userAccess)
	}
	resourceControl.UserAccesses = userAccesses

	var teamAccesses = make([]api.TeamResourceAccess, 0)
	for _, v := range req.Teams {
		teamAccess := api.TeamResourceAccess{
			TeamID:      api.TeamID(v),
			AccessLevel: api.ReadWriteAccessLevel,
		}
		teamAccesses = append(teamAccesses, teamAccess)
	}
	resourceControl.TeamAccesses = teamAccesses

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	if !security.AuthorizedResourceControlUpdate(resourceControl, securityContext) {
		httperror.WriteErrorResponse(w, api.ErrResourceAccessDenied, http.StatusForbidden, handler.Logger)
		return
	}

	err = handler.ResourceControlService.UpdateResourceControl(resourceControl.ID, resourceControl)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
}

// handleDeleteResources handles DELETE requests on /resources/:id
func (handler *ResourceHandler) handleDeleteResources(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	resourceControlID, err := strconv.Atoi(id)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	resourceControl, err := handler.ResourceControlService.ResourceControl(api.ResourceControlID(resourceControlID))

	if err == api.ErrResourceControlNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	if !security.AuthorizedResourceControlDeletion(resourceControl, securityContext) {
		httperror.WriteErrorResponse(w, api.ErrResourceAccessDenied, http.StatusForbidden, handler.Logger)
		return
	}

	err = handler.ResourceControlService.DeleteResourceControl(api.ResourceControlID(resourceControlID))
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
}
