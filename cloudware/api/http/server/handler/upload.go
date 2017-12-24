package handler

import (
	"cloudware/cloudware/api"
	httperror "cloudware/cloudware/api/http/server/error"
	"cloudware/cloudware/api/http/server/security"

	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// UploadHandler represents an HTTP API handler for managing file uploads.
type UploadHandler struct {
	*mux.Router
	Logger      *log.Logger
	FileService api.FileService
}

// NewUploadHandler returns a new instance of UploadHandler.
func NewUploadHandler(bouncer *security.RequestBouncer) *UploadHandler {
	h := &UploadHandler{
		Router: mux.NewRouter(),
		Logger: log.New(os.Stderr, "", log.LstdFlags),
	}
	h.Handle("/upload/tls/{certificate:(?:ca|cert|key)}",
		bouncer.AdministratorAccess(http.HandlerFunc(h.handlePostUploadTLS))).Methods(http.MethodPost)
	return h
}

// handlePostUploadTLS handles POST requests on /upload/tls/{certificate:(?:ca|cert|key)}?folder=<folder>
func (handler *UploadHandler) handlePostUploadTLS(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	certificate := vars["certificate"]

	folder := r.FormValue("folder")
	if folder == "" {
		httperror.WriteErrorResponse(w, ErrInvalidQueryFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	file, _, err := r.FormFile("file")
	defer file.Close()
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	var fileType api.TLSFileType
	switch certificate {
	case "ca":
		fileType = api.TLSFileCA
	case "cert":
		fileType = api.TLSFileCert
	case "key":
		fileType = api.TLSFileKey
	default:
		httperror.WriteErrorResponse(w, api.ErrUndefinedTLSFileType, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.FileService.StoreTLSFile(folder, fileType, file)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
}
