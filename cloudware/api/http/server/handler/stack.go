package handler

import (
	"encoding/json"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/asaskevich/govalidator"
	"cloudware/cloudware/api"
	"cloudware/cloudware/api/file"
	httperror "cloudware/cloudware/api/http/server/error"
	"cloudware/cloudware/api/http/server/proxy"
	"cloudware/cloudware/api/http/server/security"

	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// StackHandler represents an HTTP API handler for managing Stack.
type StackHandler struct {
	stackCreationMutex *sync.Mutex
	stackDeletionMutex *sync.Mutex
	*mux.Router
	Logger                 *log.Logger
	FileService            api.FileService
	GitService             api.GitService
	StackService           api.StackService
	EndpointService        api.EndpointService
	ResourceControlService api.ResourceControlService
	RegistryService        api.RegistryService
	DockerHubService       api.DockerHubService
	StackManager           api.StackManager
}

// NewStackHandler returns a new instance of StackHandler.
func NewStackHandler(bouncer *security.RequestBouncer) *StackHandler {
	h := &StackHandler{
		Router:             mux.NewRouter(),
		stackCreationMutex: &sync.Mutex{},
		stackDeletionMutex: &sync.Mutex{},
		Logger:             log.New(os.Stderr, "", log.LstdFlags),
	}
	h.Handle("/{endpointId}/stacks",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handlePostStacks))).Methods(http.MethodPost)
	h.Handle("/{endpointId}/stacks",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handleGetStacks))).Methods(http.MethodGet)
	h.Handle("/{endpointId}/stacks/{id}",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handleGetStack))).Methods(http.MethodGet)
	h.Handle("/{endpointId}/stacks/{id}",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handleDeleteStack))).Methods(http.MethodDelete)
	h.Handle("/{endpointId}/stacks/{id}",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handlePutStack))).Methods(http.MethodPut)
	h.Handle("/{endpointId}/stacks/{id}/stackfile",
		bouncer.RestrictedAccess(http.HandlerFunc(h.handleGetStackFile))).Methods(http.MethodGet)
	return h
}

type (
	postStacksRequest struct {
		Name             string           `valid:"required"`
		SwarmID          string           `valid:"required"`
		StackFileContent string           `valid:""`
		GitRepository    string           `valid:""`
		PathInRepository string           `valid:""`
		Env              []api.Pair `valid:""`
	}
	postStacksResponse struct {
		ID string `json:"Id"`
	}
	getStackFileResponse struct {
		StackFileContent string `json:"StackFileContent"`
	}
	putStackRequest struct {
		StackFileContent string           `valid:"required"`
		Env              []api.Pair `valid:""`
	}
)

// handlePostStacks handles POST requests on /:endpointId/stacks?method=<method>
func (handler *StackHandler) handlePostStacks(w http.ResponseWriter, r *http.Request) {
	method := r.FormValue("method")
	if method == "" {
		httperror.WriteErrorResponse(w, ErrInvalidQueryFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	if method == "string" {
		handler.handlePostStacksStringMethod(w, r)
	} else if method == "repository" {
		handler.handlePostStacksRepositoryMethod(w, r)
	} else if method == "file" {
		handler.handlePostStacksFileMethod(w, r)
	} else {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}
}

func (handler *StackHandler) handlePostStacksStringMethod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}
	endpointID := api.EndpointID(id)

	endpoint, err := handler.EndpointService.Endpoint(endpointID)
	if err == api.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	var req postStacksRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidJSON, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err = govalidator.ValidateStruct(req)
	if err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	stackName := req.Name
	if stackName == "" {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	stackFileContent := req.StackFileContent
	if stackFileContent == "" {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	swarmID := req.SwarmID
	if swarmID == "" {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	stacks, err := handler.StackService.Stacks()
	if err != nil && err != api.ErrStackNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	for _, stack := range stacks {
		if strings.EqualFold(stack.Name, stackName) {
			httperror.WriteErrorResponse(w, api.ErrStackAlreadyExists, http.StatusConflict, handler.Logger)
			return
		}
	}

	stack := &api.Stack{
		ID:         api.StackID(stackName + "_" + swarmID),
		Name:       stackName,
		SwarmID:    swarmID,
		EntryPoint: file.ComposeFileDefaultName,
		Env:        req.Env,
	}

	projectPath, err := handler.FileService.StoreStackFileFromString(string(stack.ID), stackFileContent)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
	stack.ProjectPath = projectPath

	err = handler.StackService.CreateStack(stack)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	dockerhub, err := handler.DockerHubService.DockerHub()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	registries, err := handler.RegistryService.Registries()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	filteredRegistries, err := security.FilterRegistries(registries, securityContext)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.deployStack(endpoint, stack, dockerhub, filteredRegistries)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	encodeJSON(w, &postStacksResponse{ID: string(stack.ID)}, handler.Logger)
}

func (handler *StackHandler) handlePostStacksRepositoryMethod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}
	endpointID := api.EndpointID(id)

	endpoint, err := handler.EndpointService.Endpoint(endpointID)
	if err == api.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	var req postStacksRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidJSON, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err = govalidator.ValidateStruct(req)
	if err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	stackName := req.Name
	if stackName == "" {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	swarmID := req.SwarmID
	if swarmID == "" {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	if req.GitRepository == "" {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	if req.PathInRepository == "" {
		req.PathInRepository = file.ComposeFileDefaultName
	}

	stacks, err := handler.StackService.Stacks()
	if err != nil && err != api.ErrStackNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	for _, stack := range stacks {
		if strings.EqualFold(stack.Name, stackName) {
			httperror.WriteErrorResponse(w, api.ErrStackAlreadyExists, http.StatusConflict, handler.Logger)
			return
		}
	}

	stack := &api.Stack{
		ID:         api.StackID(stackName + "_" + swarmID),
		Name:       stackName,
		SwarmID:    swarmID,
		EntryPoint: req.PathInRepository,
		Env:        req.Env,
	}

	projectPath := handler.FileService.GetStackProjectPath(string(stack.ID))
	stack.ProjectPath = projectPath

	// Ensure projectPath is empty
	err = handler.FileService.RemoveDirectory(projectPath)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.GitService.CloneRepository(req.GitRepository, projectPath)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.StackService.CreateStack(stack)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	dockerhub, err := handler.DockerHubService.DockerHub()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	registries, err := handler.RegistryService.Registries()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	filteredRegistries, err := security.FilterRegistries(registries, securityContext)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.deployStack(endpoint, stack, dockerhub, filteredRegistries)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	encodeJSON(w, &postStacksResponse{ID: string(stack.ID)}, handler.Logger)
}

func (handler *StackHandler) handlePostStacksFileMethod(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}
	endpointID := api.EndpointID(id)

	endpoint, err := handler.EndpointService.Endpoint(endpointID)
	if err == api.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	stackName := r.FormValue("Name")
	if stackName == "" {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	swarmID := r.FormValue("SwarmID")
	if swarmID == "" {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	envParam := r.FormValue("Env")
	var env []api.Pair
	if err = json.Unmarshal([]byte(envParam), &env); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	stackFile, _, err := r.FormFile("file")
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
	defer stackFile.Close()

	stacks, err := handler.StackService.Stacks()
	if err != nil && err != api.ErrStackNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	for _, stack := range stacks {
		if strings.EqualFold(stack.Name, stackName) {
			httperror.WriteErrorResponse(w, api.ErrStackAlreadyExists, http.StatusConflict, handler.Logger)
			return
		}
	}

	stack := &api.Stack{
		ID:         api.StackID(stackName + "_" + swarmID),
		Name:       stackName,
		SwarmID:    swarmID,
		EntryPoint: file.ComposeFileDefaultName,
		Env:        env,
	}

	projectPath, err := handler.FileService.StoreStackFileFromReader(string(stack.ID), stackFile)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
	stack.ProjectPath = projectPath

	err = handler.StackService.CreateStack(stack)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	dockerhub, err := handler.DockerHubService.DockerHub()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	registries, err := handler.RegistryService.Registries()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	filteredRegistries, err := security.FilterRegistries(registries, securityContext)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.deployStack(endpoint, stack, dockerhub, filteredRegistries)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	encodeJSON(w, &postStacksResponse{ID: string(stack.ID)}, handler.Logger)
}

// handleGetStacks handles GET requests on /:endpointId/stacks?swarmId=<swarmId>
func (handler *StackHandler) handleGetStacks(w http.ResponseWriter, r *http.Request) {
	swarmID := r.FormValue("swarmId")

	vars := mux.Vars(r)

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	id, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}
	endpointID := api.EndpointID(id)

	_, err = handler.EndpointService.Endpoint(endpointID)
	if err == api.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	var stacks []api.Stack
	if swarmID == "" {
		stacks, err = handler.StackService.Stacks()
	} else {
		stacks, err = handler.StackService.StacksBySwarmID(swarmID)
	}
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	resourceControls, err := handler.ResourceControlService.ResourceControls()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	filteredStacks := proxy.FilterStacks(stacks, resourceControls, securityContext.IsAdmin,
		securityContext.UserID, securityContext.UserMemberships)

	encodeJSON(w, filteredStacks, handler.Logger)
}

// handleGetStack handles GET requests on /:endpointId/stacks/:id
func (handler *StackHandler) handleGetStack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["id"]

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	endpointID, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err = handler.EndpointService.Endpoint(api.EndpointID(endpointID))
	if err == api.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	stack, err := handler.StackService.Stack(api.StackID(stackID))
	if err == api.ErrStackNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	resourceControl, err := handler.ResourceControlService.ResourceControlByResourceID(stack.Name)
	if err != nil && err != api.ErrResourceControlNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	extendedStack := proxy.ExtendedStack{*stack, api.ResourceControl{}}
	if resourceControl != nil {
		if securityContext.IsAdmin || proxy.CanAccessStack(stack, resourceControl, securityContext.UserID, securityContext.UserMemberships) {
			extendedStack.ResourceControl = *resourceControl
		} else {
			httperror.WriteErrorResponse(w, api.ErrResourceAccessDenied, http.StatusForbidden, handler.Logger)
			return
		}
	}

	encodeJSON(w, extendedStack, handler.Logger)
}

// handlePutStack handles PUT requests on /:endpointId/stacks/:id
func (handler *StackHandler) handlePutStack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["id"]

	endpointID, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	endpoint, err := handler.EndpointService.Endpoint(api.EndpointID(endpointID))
	if err == api.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	stack, err := handler.StackService.Stack(api.StackID(stackID))
	if err == api.ErrStackNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	var req putStackRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidJSON, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err = govalidator.ValidateStruct(req)
	if err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}
	stack.Env = req.Env

	_, err = handler.FileService.StoreStackFileFromString(string(stack.ID), req.StackFileContent)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.StackService.UpdateStack(stack.ID, stack)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	securityContext, err := security.RetrieveRestrictedRequestContext(r)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	dockerhub, err := handler.DockerHubService.DockerHub()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	registries, err := handler.RegistryService.Registries()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	filteredRegistries, err := security.FilterRegistries(registries, securityContext)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.deployStack(endpoint, stack, dockerhub, filteredRegistries)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
}

// handleGetStackFile handles GET requests on /:endpointId/stacks/:id/stackfile
func (handler *StackHandler) handleGetStackFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["id"]

	endpointID, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err = handler.EndpointService.Endpoint(api.EndpointID(endpointID))
	if err == api.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	stack, err := handler.StackService.Stack(api.StackID(stackID))
	if err == api.ErrStackNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	stackFileContent, err := handler.FileService.GetFileContent(path.Join(stack.ProjectPath, stack.EntryPoint))
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	encodeJSON(w, &getStackFileResponse{StackFileContent: stackFileContent}, handler.Logger)
}

// handleDeleteStack handles DELETE requests on /:endpointId/stacks/:id
func (handler *StackHandler) handleDeleteStack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stackID := vars["id"]

	endpointID, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	endpoint, err := handler.EndpointService.Endpoint(api.EndpointID(endpointID))
	if err == api.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	stack, err := handler.StackService.Stack(api.StackID(stackID))
	if err == api.ErrStackNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	handler.stackDeletionMutex.Lock()
	err = handler.StackManager.Remove(stack, endpoint)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
	handler.stackDeletionMutex.Unlock()

	err = handler.StackService.DeleteStack(api.StackID(stackID))
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.FileService.RemoveDirectory(stack.ProjectPath)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
}

func (handler *StackHandler) deployStack(endpoint *api.Endpoint, stack *api.Stack, dockerhub *api.DockerHub, registries []api.Registry) error {
	handler.stackCreationMutex.Lock()

	err := handler.StackManager.Login(dockerhub, registries, endpoint)
	if err != nil {
		handler.stackCreationMutex.Unlock()
		return err
	}

	err = handler.StackManager.Deploy(stack, endpoint)
	if err != nil {
		handler.stackCreationMutex.Unlock()
		return err
	}

	err = handler.StackManager.Logout(endpoint)
	if err != nil {
		handler.stackCreationMutex.Unlock()
		return err
	}

	handler.stackCreationMutex.Unlock()
	return nil
}
