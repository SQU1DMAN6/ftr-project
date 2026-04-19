package repository

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	repoStore "inkdrop/repository"
	viewBackend "inkdrop/view/connector"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type documentEditMutationRequest struct {
	Op           string             `json:"op"`
	ClientID     string             `json:"clientId"`
	Version      int64              `json:"version"`
	Selection    *liveEditSelection `json:"selection,omitempty"`
	SavePath     string             `json:"savePath,omitempty"`
	Overwrite    bool               `json:"overwrite,omitempty"`
	DocumentData string             `json:"documentData,omitempty"`
}

type documentEditMutationResponse struct {
	Success   bool               `json:"success"`
	Version   int64              `json:"version,omitempty"`
	Path      string             `json:"path,omitempty"`
	FileURL   string             `json:"fileUrl,omitempty"`
	SavedAt   int64              `json:"savedAt,omitempty"`
	Presence  []liveEditPresence `json:"presence,omitempty"`
	EditURL   string             `json:"editURL,omitempty"`
	Message   string             `json:"message,omitempty"`
	Error     string             `json:"error,omitempty"`
	Timestamp int64              `json:"timestamp,omitempty"`
}

func isDocumentEditorTarget(target *liveEditTarget) bool {
	return strings.EqualFold(path.Ext(strings.TrimSpace(target.FileName)), ".docx")
}

func handleDocumentEditGet(w http.ResponseWriter, r *http.Request, target *liveEditTarget, sessionUser string) {
	statusCode := http.StatusOK
	err := hydrateLiveEditTarget(target, sessionUser)
	if err != nil {
		if status, ok := err.(liveEditHTTPError); ok {
			statusCode = status.StatusCode
			err = status.Err
		} else {
			statusCode = http.StatusInternalServerError
		}
	}

	if r.URL.Query().Get("events") == "1" {
		if err != nil {
			http.Error(w, err.Error(), statusCode)
			return
		}
		streamDocumentEditEvents(w, r, target, sessionUser)
		return
	}

	if r.URL.Query().Get("content") == "1" {
		if err != nil {
			writeJSON(w, statusCode, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		serveDocumentEditContent(w, target)
		return
	}

	var initialVersion int64
	var initialSavedAt int64
	var initialFileURL string
	var snapshotErr error
	if err == nil && target.FileSize <= documentEditMaxFileSize {
		var snapshot documentEditEvent
		snapshot, snapshotErr = documentEditSessions.Snapshot(target)
		if snapshotErr == nil {
			initialVersion = snapshot.Version
			initialSavedAt = snapshot.SavedAt
			initialFileURL = versionedDocumentFileURL(snapshot.FileURL, snapshot.Version)
		}
	}

	params := viewBackend.FrontEndParams{
		Title:                   fmt.Sprintf("Document Edit - %s", target.FileName),
		Name:                    sessionUser,
		Error:                   make(map[string]string),
		Path:                    target.WorkingDir,
		EditorFileName:          target.FileName,
		EditorFilePath:          target.DisplayPath,
		EditorRepoOwner:         target.RepoOwner,
		EditorRepoName:          target.RepoName,
		EditorBackURL:           target.BackURL,
		EditorEditable:          err == nil && snapshotErr == nil && target.FileSize <= documentEditMaxFileSize,
		DocumentEditorFileURL:   initialFileURL,
		DocumentEditorLoadURL:   target.LoadURL,
		DocumentEditorSyncURL:   target.EditURL,
		DocumentEditorStreamURL: target.StreamURL,
		DocumentEditorSaveAsURL: target.EditURL,
		DocumentEditorVersion:   initialVersion,
		DocumentEditorSavedAt:   initialSavedAt,
	}

	if err != nil {
		params.Error["general"] = err.Error()
	} else if snapshotErr != nil {
		params.Error["general"] = fmt.Sprintf("Failed to open the document editor: %s", snapshotErr)
	} else if target.FileSize > documentEditMaxFileSize {
		params.Error["general"] = fmt.Sprintf(
			"This DOCX file is %d bytes. Document editing currently supports files up to %d bytes.",
			target.FileSize,
			documentEditMaxFileSize,
		)
	}

	if len(params.Error) > 0 {
		w.WriteHeader(statusCode)
	}

	if renderErr := viewBackend.DocumentEditFile(w, params); renderErr != nil {
		http.Error(w, fmt.Sprintf("Failed to render document editor: %s", renderErr), http.StatusInternalServerError)
	}
}

func handleDocumentEditMutation(w http.ResponseWriter, r *http.Request, target *liveEditTarget, sessionUser string) {
	statusCode := http.StatusOK
	err := hydrateLiveEditTarget(target, sessionUser)
	if err != nil {
		if status, ok := err.(liveEditHTTPError); ok {
			statusCode = status.StatusCode
			err = status.Err
		} else {
			statusCode = http.StatusInternalServerError
		}
		writeJSON(w, statusCode, documentEditMutationResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, (documentEditMaxFileSize*2)+(1<<20))
	var req documentEditMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, documentEditMutationResponse{
			Success: false,
			Error:   "invalid document-edit request payload",
		})
		return
	}

	req.Op = strings.ToLower(strings.TrimSpace(req.Op))
	req.ClientID = strings.TrimSpace(req.ClientID)
	if req.ClientID == "" {
		writeJSON(w, http.StatusBadRequest, documentEditMutationResponse{
			Success: false,
			Error:   "clientId is required",
		})
		return
	}
	if req.Op == "" {
		req.Op = "sync"
	}

	switch req.Op {
	case "presence":
		event, err := documentEditSessions.UpdatePresence(target, req.ClientID, sessionUser, req.Selection)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, documentEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, documentEditMutationResponse{
			Success:   true,
			Version:   event.Version,
			Presence:  event.Presence,
			Timestamp: event.Timestamp,
		})
	case "leave":
		event := documentEditSessions.Leave(target, req.ClientID)
		writeJSON(w, http.StatusOK, documentEditMutationResponse{
			Success:   true,
			Version:   event.Version,
			Presence:  event.Presence,
			Timestamp: event.Timestamp,
		})
	case "sync", "save":
		data, err := decodeDocumentPayload(req.DocumentData)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, documentEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		event, err := documentEditSessions.UpdateContent(target, req.ClientID, sessionUser, data, req.Selection)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, documentEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, documentEditMutationResponse{
			Success:   true,
			Version:   event.Version,
			Path:      event.Path,
			FileURL:   versionedDocumentFileURL(event.FileURL, event.Version),
			SavedAt:   event.SavedAt,
			Presence:  event.Presence,
			Message:   "saved",
			Timestamp: event.Timestamp,
		})
	case "save_as":
		data, err := decodeDocumentPayload(req.DocumentData)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, documentEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		savePath := normalizeBrowserPath(req.SavePath)
		if savePath == "/" {
			writeJSON(w, http.StatusBadRequest, documentEditMutationResponse{
				Success: false,
				Error:   "save path is required",
			})
			return
		}

		if savePath == target.DisplayPath {
			event, err := documentEditSessions.UpdateContent(target, req.ClientID, sessionUser, data, req.Selection)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, documentEditMutationResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			writeJSON(w, http.StatusOK, documentEditMutationResponse{
				Success:   true,
				Version:   event.Version,
				Path:      event.Path,
				FileURL:   versionedDocumentFileURL(event.FileURL, event.Version),
				SavedAt:   event.SavedAt,
				Presence:  event.Presence,
				EditURL:   target.EditURL,
				Message:   "saved",
				Timestamp: event.Timestamp,
			})
			return
		}

		filePath, err := repoStore.WriteFileAtRepoPath(target.RepoOwner, target.RepoName, savePath, data, req.Overwrite)
		if err != nil {
			if errors.Is(err, repoStore.ErrItemExists) {
				writeJSON(w, http.StatusConflict, documentEditMutationResponse{
					Success: false,
					Error:   "a file already exists at that path",
				})
				return
			}
			writeJSON(w, http.StatusBadRequest, documentEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		savedTarget := buildLiveEditTargetForRepoPath(target.RepoOwner, target.RepoName, savePath, filePath)
		snapshot, err := documentEditSessions.Snapshot(savedTarget)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, documentEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, documentEditMutationResponse{
			Success:   true,
			Version:   snapshot.Version,
			Path:      snapshot.Path,
			FileURL:   versionedDocumentFileURL(snapshot.FileURL, snapshot.Version),
			SavedAt:   snapshot.SavedAt,
			Presence:  snapshot.Presence,
			EditURL:   savedTarget.EditURL,
			Message:   "saved_as",
			Timestamp: snapshot.Timestamp,
		})
	default:
		writeJSON(w, http.StatusBadRequest, documentEditMutationResponse{
			Success: false,
			Error:   "unsupported document-edit operation",
		})
	}
}

func serveDocumentEditContent(w http.ResponseWriter, target *liveEditTarget) {
	if target.FileSize > documentEditMaxFileSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("File exceeds the %d byte document-edit limit.", documentEditMaxFileSize),
		})
		return
	}

	snapshot, err := documentEditSessions.Snapshot(target)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    snapshot.Path,
		"fileUrl": versionedDocumentFileURL(snapshot.FileURL, snapshot.Version),
		"size":    target.FileSize,
		"version": snapshot.Version,
		"savedAt": snapshot.SavedAt,
	})
}

func streamDocumentEditEvents(w http.ResponseWriter, r *http.Request, target *liveEditTarget, sessionUser string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming is not supported in this environment.", http.StatusInternalServerError)
		return
	}

	clientID := strings.TrimSpace(r.URL.Query().Get("client"))
	if clientID == "" {
		http.Error(w, "client query parameter is required", http.StatusBadRequest)
		return
	}

	_, events, cancel, err := documentEditSessions.Subscribe(target, clientID, sessionUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cancel()

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			event.FileURL = versionedDocumentFileURL(event.FileURL, event.Version)
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func decodeDocumentPayload(data string) ([]byte, error) {
	payload := strings.TrimSpace(data)
	if payload == "" {
		return nil, errors.New("documentData is required")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, errors.New("documentData must be valid base64")
	}
	if int64(len(decoded)) > documentEditMaxFileSize {
		return nil, fmt.Errorf("document exceeds the %d byte document-edit limit", documentEditMaxFileSize)
	}
	return decoded, nil
}

func versionedDocumentFileURL(rawURL string, version int64) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set("v", fmt.Sprintf("%d", version))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
