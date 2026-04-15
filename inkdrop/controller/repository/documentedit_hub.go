package repository

import (
	"errors"
	repoStore "inkdrop/repository"
	"os"
	"sort"
	"sync"
	"time"
)

const documentEditMaxFileSize int64 = 25 * 1024 * 1024

var documentEditSessions = newDocumentEditSessionHub()

type documentEditEvent struct {
	Type      string             `json:"type"`
	ClientID  string             `json:"clientId,omitempty"`
	User      string             `json:"user,omitempty"`
	Version   int64              `json:"version,omitempty"`
	Path      string             `json:"path,omitempty"`
	FileName  string             `json:"fileName,omitempty"`
	FileURL   string             `json:"fileUrl,omitempty"`
	SavedAt   int64              `json:"savedAt,omitempty"`
	External  bool               `json:"external,omitempty"`
	Presence  []liveEditPresence `json:"presence,omitempty"`
	Message   string             `json:"message,omitempty"`
	Timestamp int64              `json:"timestamp"`
}

type documentEditDocument struct {
	Key             string
	FilePath        string
	FileName        string
	DisplayPath     string
	SourceURL       string
	Version         int64
	LastSavedAt     time.Time
	LastDiskModTime time.Time
	LastDiskSize    int64
	LastActiveAt    time.Time
	Subscribers     map[string]chan documentEditEvent
	Presence        map[string]liveEditPresence
	WatcherRunning  bool
}

type documentEditSessionHub struct {
	mu        sync.Mutex
	documents map[string]*documentEditDocument
}

func newDocumentEditSessionHub() *documentEditSessionHub {
	return &documentEditSessionHub{
		documents: make(map[string]*documentEditDocument),
	}
}

func (h *documentEditSessionHub) Snapshot(target *liveEditTarget) (documentEditEvent, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		return documentEditEvent{}, err
	}

	return h.snapshotEventLocked(doc, "", "", false), nil
}

func (h *documentEditSessionHub) Subscribe(target *liveEditTarget, clientID string, user string) (documentEditEvent, <-chan documentEditEvent, func(), error) {
	h.mu.Lock()
	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		h.mu.Unlock()
		return documentEditEvent{}, nil, nil, err
	}

	if existing, ok := doc.Subscribers[clientID]; ok {
		delete(doc.Subscribers, clientID)
		close(existing)
	}

	ch := make(chan documentEditEvent, 16)
	doc.Subscribers[clientID] = ch
	h.updatePresenceLocked(doc, clientID, user, nil)
	doc.LastActiveAt = time.Now()
	snapshot := h.snapshotEventLocked(doc, "", "", false)
	presenceEvent := h.presenceEventLocked(doc, "", "")
	channels := h.subscriberChannelsLocked(doc)
	if !doc.WatcherRunning {
		doc.WatcherRunning = true
		go h.watchDocument(doc.Key)
	}
	h.mu.Unlock()

	h.enqueueEvent(ch, snapshot)
	h.sendToChannels(channels, presenceEvent)

	cancel := func() {
		h.unsubscribe(target.FilePath, clientID)
	}

	return snapshot, ch, cancel, nil
}

func (h *documentEditSessionHub) UpdateContent(target *liveEditTarget, clientID string, user string, data []byte, selection *liveEditSelection) (documentEditEvent, error) {
	if int64(len(data)) > documentEditMaxFileSize {
		return documentEditEvent{}, errors.New("document exceeds the DOCX editing size limit")
	}

	h.mu.Lock()
	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		h.mu.Unlock()
		return documentEditEvent{}, err
	}

	doc.LastActiveAt = time.Now()
	h.updatePresenceLocked(doc, clientID, user, selection)

	_, err = repoStore.WriteFile(target.RepoOwner, target.RepoName, target.WorkingDir, target.FileName, data)
	if err != nil {
		h.mu.Unlock()
		return documentEditEvent{}, err
	}

	info, statErr := os.Stat(target.FilePath)
	if statErr == nil {
		doc.LastDiskModTime = info.ModTime()
		doc.LastDiskSize = info.Size()
	}
	doc.Version++
	doc.LastSavedAt = time.Now()
	event := h.snapshotEventLocked(doc, clientID, user, false)
	channels := h.subscriberChannelsLocked(doc)
	h.mu.Unlock()

	h.sendToChannels(channels, event)
	return event, nil
}

func (h *documentEditSessionHub) UpdatePresence(target *liveEditTarget, clientID string, user string, selection *liveEditSelection) (documentEditEvent, error) {
	h.mu.Lock()
	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		h.mu.Unlock()
		return documentEditEvent{}, err
	}

	doc.LastActiveAt = time.Now()
	h.updatePresenceLocked(doc, clientID, user, selection)
	event := h.presenceEventLocked(doc, clientID, user)
	channels := h.subscriberChannelsLocked(doc)
	h.mu.Unlock()

	h.sendToChannels(channels, event)
	return event, nil
}

func (h *documentEditSessionHub) Leave(target *liveEditTarget, clientID string) documentEditEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	doc, ok := h.documents[target.FilePath]
	if !ok {
		return documentEditEvent{
			Type:      "presence",
			Timestamp: time.Now().UnixMilli(),
		}
	}

	if ch, ok := doc.Subscribers[clientID]; ok {
		delete(doc.Subscribers, clientID)
		close(ch)
	}
	delete(doc.Presence, clientID)
	doc.LastActiveAt = time.Now()
	event := h.presenceEventLocked(doc, "", "")
	channels := h.subscriberChannelsLocked(doc)
	go h.sendToChannels(channels, event)
	return event
}

func (h *documentEditSessionHub) unsubscribe(documentKey string, clientID string) {
	h.mu.Lock()
	doc, ok := h.documents[documentKey]
	if !ok {
		h.mu.Unlock()
		return
	}

	if ch, ok := doc.Subscribers[clientID]; ok {
		delete(doc.Subscribers, clientID)
		close(ch)
	}
	delete(doc.Presence, clientID)
	doc.LastActiveAt = time.Now()
	event := h.presenceEventLocked(doc, "", "")
	channels := h.subscriberChannelsLocked(doc)
	h.mu.Unlock()

	h.sendToChannels(channels, event)
}

func (h *documentEditSessionHub) watchDocument(documentKey string) {
	ticker := time.NewTicker(liveEditDiskPollInterval)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.Lock()
		doc, ok := h.documents[documentKey]
		if !ok {
			h.mu.Unlock()
			return
		}

		if len(doc.Subscribers) == 0 && time.Since(doc.LastActiveAt) > liveEditSessionIdleTTL {
			delete(h.documents, documentKey)
			h.mu.Unlock()
			return
		}

		filePath := doc.FilePath
		lastDiskModTime := doc.LastDiskModTime
		lastDiskSize := doc.LastDiskSize
		presenceChanged := h.pruneStalePresenceLocked(doc, time.Now())
		presenceEvent := h.presenceEventLocked(doc, "", "")
		presenceChannels := h.subscriberChannelsLocked(doc)
		h.mu.Unlock()

		if presenceChanged {
			h.sendToChannels(presenceChannels, presenceEvent)
		}

		info, err := os.Stat(filePath)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if info.ModTime().Equal(lastDiskModTime) && info.Size() == lastDiskSize {
			continue
		}

		h.mu.Lock()
		doc, ok = h.documents[documentKey]
		if !ok {
			h.mu.Unlock()
			return
		}

		doc.Version++
		doc.LastSavedAt = info.ModTime()
		doc.LastDiskModTime = info.ModTime()
		doc.LastDiskSize = info.Size()
		doc.LastActiveAt = time.Now()
		event := h.snapshotEventLocked(doc, "", "", true)
		channels := h.subscriberChannelsLocked(doc)
		h.mu.Unlock()

		h.sendToChannels(channels, event)
	}
}

func (h *documentEditSessionHub) ensureDocumentLocked(target *liveEditTarget) (*documentEditDocument, error) {
	if doc, ok := h.documents[target.FilePath]; ok {
		if err := h.refreshDocumentFromDiskLocked(doc); err != nil {
			return nil, err
		}
		return doc, nil
	}

	info, err := os.Stat(target.FilePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("only regular files can be opened in document edit")
	}

	doc := &documentEditDocument{
		Key:             target.FilePath,
		FilePath:        target.FilePath,
		FileName:        target.FileName,
		DisplayPath:     target.DisplayPath,
		SourceURL:       target.SourceURL,
		Version:         1,
		LastSavedAt:     info.ModTime(),
		LastDiskModTime: info.ModTime(),
		LastDiskSize:    info.Size(),
		LastActiveAt:    time.Now(),
		Subscribers:     make(map[string]chan documentEditEvent),
		Presence:        make(map[string]liveEditPresence),
	}
	h.documents[target.FilePath] = doc
	return doc, nil
}

func (h *documentEditSessionHub) refreshDocumentFromDiskLocked(doc *documentEditDocument) error {
	info, err := os.Stat(doc.FilePath)
	if err != nil || !info.Mode().IsRegular() {
		return err
	}
	if info.ModTime().Equal(doc.LastDiskModTime) && info.Size() == doc.LastDiskSize {
		return nil
	}
	doc.Version++
	doc.LastSavedAt = info.ModTime()
	doc.LastDiskModTime = info.ModTime()
	doc.LastDiskSize = info.Size()
	return nil
}

func (h *documentEditSessionHub) updatePresenceLocked(doc *documentEditDocument, clientID string, user string, selection *liveEditSelection) {
	presence := doc.Presence[clientID]
	presence.ClientID = clientID
	presence.User = user
	presence.Color = liveEditPresenceColor(clientID)
	presence.Selection = selection
	presence.UpdatedAt = time.Now().UnixMilli()
	doc.Presence[clientID] = presence
}

func (h *documentEditSessionHub) snapshotEventLocked(doc *documentEditDocument, clientID string, user string, external bool) documentEditEvent {
	return documentEditEvent{
		Type:      "snapshot",
		ClientID:  clientID,
		User:      user,
		Version:   doc.Version,
		Path:      doc.DisplayPath,
		FileName:  doc.FileName,
		FileURL:   doc.SourceURL,
		SavedAt:   doc.LastSavedAt.UnixMilli(),
		External:  external,
		Presence:  h.presenceListLocked(doc),
		Timestamp: time.Now().UnixMilli(),
	}
}

func (h *documentEditSessionHub) presenceEventLocked(doc *documentEditDocument, clientID string, user string) documentEditEvent {
	return documentEditEvent{
		Type:      "presence",
		ClientID:  clientID,
		User:      user,
		Version:   doc.Version,
		Presence:  h.presenceListLocked(doc),
		Timestamp: time.Now().UnixMilli(),
	}
}

func (h *documentEditSessionHub) presenceListLocked(doc *documentEditDocument) []liveEditPresence {
	presence := make([]liveEditPresence, 0, len(doc.Presence))
	for _, entry := range doc.Presence {
		presence = append(presence, entry)
	}
	sort.Slice(presence, func(i int, j int) bool {
		if presence[i].User == presence[j].User {
			return presence[i].ClientID < presence[j].ClientID
		}
		return presence[i].User < presence[j].User
	})
	return presence
}

func (h *documentEditSessionHub) subscriberChannelsLocked(doc *documentEditDocument) []chan documentEditEvent {
	if doc == nil {
		return nil
	}
	channels := make([]chan documentEditEvent, 0, len(doc.Subscribers))
	for _, ch := range doc.Subscribers {
		channels = append(channels, ch)
	}
	return channels
}

func (h *documentEditSessionHub) sendToChannels(channels []chan documentEditEvent, event documentEditEvent) {
	for _, ch := range channels {
		h.enqueueEvent(ch, event)
	}
}

func (h *documentEditSessionHub) enqueueEvent(ch chan documentEditEvent, event documentEditEvent) {
	defer func() {
		_ = recover()
	}()

	select {
	case ch <- event:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *documentEditSessionHub) pruneStalePresenceLocked(doc *documentEditDocument, now time.Time) bool {
	if doc == nil || len(doc.Presence) == 0 {
		return false
	}

	cutoff := now.Add(-liveEditPresenceTTL).UnixMilli()
	changed := false
	for clientID, presence := range doc.Presence {
		if presence.UpdatedAt >= cutoff {
			continue
		}
		if ch, subscribed := doc.Subscribers[clientID]; subscribed {
			delete(doc.Subscribers, clientID)
			close(ch)
		}
		delete(doc.Presence, clientID)
		changed = true
	}
	return changed
}
