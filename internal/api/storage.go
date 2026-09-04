package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/mountd"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
)

// drivesResponse always returns 200 even when the storage helper isn't
// installed -- HelperAvailable false is how the dashboard tells "nothing
// plugged in / not set up on this box" apart from a real error, matching
// the rest of the dashboard's convention of not surfacing an optional
// feature's absence as an error banner.
type drivesResponse struct {
	HelperAvailable bool           `json:"helper_available"`
	Drives          []mountd.Drive `json:"drives"`
}

func (s *Server) listDrives(w http.ResponseWriter, r *http.Request) {
	drives, err := s.Orchestrator.ListDrives(r.Context())
	if errors.Is(err, mountd.ErrUnavailable) {
		writeJSON(w, http.StatusOK, drivesResponse{HelperAvailable: false, Drives: []mountd.Drive{}})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, drivesResponse{HelperAvailable: true, Drives: drives})
}

func (s *Server) mountDrive(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	drive, err := s.Orchestrator.MountDrive(r.Context(), uuid)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, drive)
}

func (s *Server) unmountDrive(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	if err := s.Orchestrator.UnmountDrive(r.Context(), uuid); err != nil {
		writeStorageError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// nasShareInfo enriches a NAS-share service with its deployment's name/slug
// so the Storage page doesn't need a second round trip per share to show
// something more useful than a bare service ID.
type nasShareInfo struct {
	DeploymentID      int64  `json:"deployment_id"`
	DeploymentName    string `json:"deployment_name"`
	DeploymentSlug    string `json:"deployment_slug"`
	ServiceID         int64  `json:"service_id"`
	Status            string `json:"status"`
	ShareName         string `json:"share_name"`
	DirectPublishPort int    `json:"direct_publish_port"`
	HostMountSource   string `json:"host_mount_source"`
}

func (s *Server) listNASShares(w http.ResponseWriter, r *http.Request) {
	shares, err := s.Store.ListNASShares(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]nasShareInfo, 0, len(shares))
	for _, svc := range shares {
		dep, err := s.Store.GetDeployment(r.Context(), svc.DeploymentID)
		if err != nil {
			s.Log.Warn("list NAS shares: failed to load deployment", "deployment_id", svc.DeploymentID, "error", err)
			continue
		}
		port := 0
		if svc.DirectPublishPort != nil {
			port = *svc.DirectPublishPort
		}
		out = append(out, nasShareInfo{
			DeploymentID:      dep.ID,
			DeploymentName:    dep.Name,
			DeploymentSlug:    dep.Slug,
			ServiceID:         svc.ID,
			Status:            dep.Status,
			ShareName:         dep.Name,
			DirectPublishPort: port,
			HostMountSource:   svc.HostMountSource,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type createNASShareRequest struct {
	DriveUUID string `json:"drive_uuid"`
	Slug      string `json:"slug"`
	ShareName string `json:"share_name"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

func (s *Server) createNASShare(w http.ResponseWriter, r *http.Request) {
	var req createNASShareRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	dep, err := s.Orchestrator.CreateNASShare(r.Context(), orchestrator.CreateNASShareParams{
		DriveUUID: req.DriveUUID,
		Slug:      req.Slug,
		ShareName: req.ShareName,
		Username:  req.Username,
		Password:  req.Password,
	})
	if err != nil {
		writeStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dep)
}

// writeStorageError distinguishes "the helper isn't installed/running"
// (503, a setup problem, not a bug) from every other storage error (502 --
// the helper is reachable but the operation itself failed, e.g. an unknown
// drive, an unsupported filesystem, or an already-shared drive).
func writeStorageError(w http.ResponseWriter, err error) {
	if errors.Is(err, mountd.ErrUnavailable) {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeError(w, http.StatusBadGateway, err.Error())
}
