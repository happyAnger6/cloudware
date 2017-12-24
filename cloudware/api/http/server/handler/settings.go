package handler

import (
	"encoding/json"

	"github.com/asaskevich/govalidator"
	"cloudware/cloudware/api"
	"cloudware/cloudware/api/file"
	httperror "cloudware/cloudware/api/http/server/error"
	"cloudware/cloudware/api/http/server/security"

	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// SettingsHandler represents an HTTP API handler for managing Settings.
type SettingsHandler struct {
	*mux.Router
	Logger          *log.Logger
	SettingsService api.SettingsService
	LDAPService     api.LDAPService
	FileService     api.FileService
}

// NewSettingsHandler returns a new instance of OldSettingsHandler.
func NewSettingsHandler(bouncer *security.RequestBouncer) *SettingsHandler {
	h := &SettingsHandler{
		Router: mux.NewRouter(),
		Logger: log.New(os.Stderr, "", log.LstdFlags),
	}
	h.Handle("/settings",
		bouncer.AdministratorAccess(http.HandlerFunc(h.handleGetSettings))).Methods(http.MethodGet)
	h.Handle("/settings",
		bouncer.AdministratorAccess(http.HandlerFunc(h.handlePutSettings))).Methods(http.MethodPut)
	h.Handle("/settings/public",
		bouncer.PublicAccess(http.HandlerFunc(h.handleGetPublicSettings))).Methods(http.MethodGet)
	h.Handle("/settings/authentication/checkLDAP",
		bouncer.AdministratorAccess(http.HandlerFunc(h.handlePutSettingsLDAPCheck))).Methods(http.MethodPut)

	return h
}

type (
	publicSettingsResponse struct {
		LogoURL                            string                         `json:"LogoURL"`
		DisplayDonationHeader              bool                           `json:"DisplayDonationHeader"`
		DisplayExternalContributors        bool                           `json:"DisplayExternalContributors"`
		AuthenticationMethod               api.AuthenticationMethod `json:"AuthenticationMethod"`
		AllowBindMountsForRegularUsers     bool                           `json:"AllowBindMountsForRegularUsers"`
		AllowPrivilegedModeForRegularUsers bool                           `json:"AllowPrivilegedModeForRegularUsers"`
	}

	putSettingsRequest struct {
		TemplatesURL                       string                 `valid:"required"`
		LogoURL                            string                 `valid:""`
		BlackListedLabels                  []api.Pair       `valid:""`
		DisplayDonationHeader              bool                   `valid:""`
		DisplayExternalContributors        bool                   `valid:""`
		AuthenticationMethod               int                    `valid:"required"`
		LDAPSettings                       api.LDAPSettings `valid:""`
		AllowBindMountsForRegularUsers     bool                   `valid:""`
		AllowPrivilegedModeForRegularUsers bool                   `valid:""`
	}

	putSettingsLDAPCheckRequest struct {
		LDAPSettings api.LDAPSettings `valid:""`
	}
)

// handleGetSettings handles GET requests on /settings
func (handler *SettingsHandler) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := handler.SettingsService.Settings()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	encodeJSON(w, settings, handler.Logger)
	return
}

// handleGetPublicSettings handles GET requests on /settings/public
func (handler *SettingsHandler) handleGetPublicSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := handler.SettingsService.Settings()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	publicSettings := &publicSettingsResponse{
		LogoURL:                            settings.LogoURL,
		DisplayDonationHeader:              settings.DisplayDonationHeader,
		DisplayExternalContributors:        settings.DisplayExternalContributors,
		AuthenticationMethod:               settings.AuthenticationMethod,
		AllowBindMountsForRegularUsers:     settings.AllowBindMountsForRegularUsers,
		AllowPrivilegedModeForRegularUsers: settings.AllowPrivilegedModeForRegularUsers,
	}

	encodeJSON(w, publicSettings, handler.Logger)
	return
}

// handlePutSettings handles PUT requests on /settings
func (handler *SettingsHandler) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req putSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidJSON, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err := govalidator.ValidateStruct(req)
	if err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	settings := &api.Settings{
		TemplatesURL:                       req.TemplatesURL,
		LogoURL:                            req.LogoURL,
		BlackListedLabels:                  req.BlackListedLabels,
		DisplayDonationHeader:              req.DisplayDonationHeader,
		DisplayExternalContributors:        req.DisplayExternalContributors,
		LDAPSettings:                       req.LDAPSettings,
		AllowBindMountsForRegularUsers:     req.AllowBindMountsForRegularUsers,
		AllowPrivilegedModeForRegularUsers: req.AllowPrivilegedModeForRegularUsers,
	}

	if req.AuthenticationMethod == 1 {
		settings.AuthenticationMethod = api.AuthenticationInternal
	} else if req.AuthenticationMethod == 2 {
		settings.AuthenticationMethod = api.AuthenticationLDAP
	} else {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	if (settings.LDAPSettings.TLSConfig.TLS || settings.LDAPSettings.StartTLS) && !settings.LDAPSettings.TLSConfig.TLSSkipVerify {
		caCertPath, _ := handler.FileService.GetPathForTLSFile(file.LDAPStorePath, api.TLSFileCA)
		settings.LDAPSettings.TLSConfig.TLSCACertPath = caCertPath
	} else {
		settings.LDAPSettings.TLSConfig.TLSCACertPath = ""
		err := handler.FileService.DeleteTLSFiles(file.LDAPStorePath)
		if err != nil {
			httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		}
	}

	err = handler.SettingsService.StoreSettings(settings)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
	}
}

// handlePutSettingsLDAPCheck handles PUT requests on /settings/ldap/check
func (handler *SettingsHandler) handlePutSettingsLDAPCheck(w http.ResponseWriter, r *http.Request) {
	var req putSettingsLDAPCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidJSON, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err := govalidator.ValidateStruct(req)
	if err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	if (req.LDAPSettings.TLSConfig.TLS || req.LDAPSettings.StartTLS) && !req.LDAPSettings.TLSConfig.TLSSkipVerify {
		caCertPath, _ := handler.FileService.GetPathForTLSFile(file.LDAPStorePath, api.TLSFileCA)
		req.LDAPSettings.TLSConfig.TLSCACertPath = caCertPath
	}

	err = handler.LDAPService.TestConnectivity(&req.LDAPSettings)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
}
