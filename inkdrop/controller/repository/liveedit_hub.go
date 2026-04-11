package repository

import (
	"bytes"
	"errors"
	"hash/fnv"
	repoStore "inkdrop/repository"
	"os"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	liveEditDiskPollInterval = 2 * time.Second
	liveEditSessionIdleTTL   = 10 * time.Minute
	liveEditPresenceTTL      = 35 * time.Second
)

var errLiveEditNonText = errors.New("file is not valid UTF-8 text")

var liveEditSessions = newLiveEditSessionHub()

type liveEditSelection struct {
	StartRow    int `json:"startRow"`
	StartColumn int `json:"startColumn"`
	EndRow      int `json:"endRow"`
	EndColumn   int `json:"endColumn"`
}

type liveEditPresence struct {
	ClientID  string             `json:"clientId"`
	User      string             `json:"user"`
	Color     string             `json:"color"`
	Selection *liveEditSelection `json:"selection,omitempty"`
	UpdatedAt int64              `json:"updatedAt"`
}

type liveEditEvent struct {
	Type      string             `json:"type"`
	ClientID  string             `json:"clientId,omitempty"`
	User      string             `json:"user,omitempty"`
	Version   int64              `json:"version,omitempty"`
	Content   string             `json:"content,omitempty"`
	Mode      string             `json:"mode,omitempty"`
	Path      string             `json:"path,omitempty"`
	FileName  string             `json:"fileName,omitempty"`
	SavedAt   int64              `json:"savedAt,omitempty"`
	External  bool               `json:"external,omitempty"`
	Presence  []liveEditPresence `json:"presence,omitempty"`
	Message   string             `json:"message,omitempty"`
	Timestamp int64              `json:"timestamp"`
}

type liveEditDocument struct {
	Key             string
	FilePath        string
	FileName        string
	DisplayPath     string
	Mode            string
	Content         string
	Version         int64
	LastSavedAt     time.Time
	LastDiskModTime time.Time
	LastActiveAt    time.Time
	Subscribers     map[string]chan liveEditEvent
	Presence        map[string]liveEditPresence
	WatcherRunning  bool
}

type liveEditSessionHub struct {
	mu        sync.Mutex
	documents map[string]*liveEditDocument
}

func newLiveEditSessionHub() *liveEditSessionHub {
	return &liveEditSessionHub{
		documents: make(map[string]*liveEditDocument),
	}
}

func (h *liveEditSessionHub) Snapshot(target *liveEditTarget) (liveEditEvent, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		return liveEditEvent{}, err
	}

	return h.snapshotEventLocked(doc, "", "", false), nil
}

func (h *liveEditSessionHub) Subscribe(target *liveEditTarget, clientID string, user string) (liveEditEvent, <-chan liveEditEvent, func(), error) {
	h.mu.Lock()
	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		h.mu.Unlock()
		return liveEditEvent{}, nil, nil, err
	}

	if existing, ok := doc.Subscribers[clientID]; ok {
		delete(doc.Subscribers, clientID)
		close(existing)
	}

	ch := make(chan liveEditEvent, 16)
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

func (h *liveEditSessionHub) UpdateContent(target *liveEditTarget, clientID string, user string, content string, selection *liveEditSelection) (liveEditEvent, error) {
	h.mu.Lock()
	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		h.mu.Unlock()
		return liveEditEvent{}, err
	}

	doc.LastActiveAt = time.Now()
	h.updatePresenceLocked(doc, clientID, user, selection)

	_, err = repoStore.WriteTextFile(target.RepoOwner, target.RepoName, target.WorkingDir, target.FileName, []byte(content))
	if err != nil {
		h.mu.Unlock()
		return liveEditEvent{}, err
	}

	info, statErr := os.Stat(target.FilePath)
	if statErr == nil {
		doc.LastDiskModTime = info.ModTime()
	}
	doc.Content = content
	doc.Version++
	doc.LastSavedAt = time.Now()
	event := h.snapshotEventLocked(doc, clientID, user, false)
	channels := h.subscriberChannelsLocked(doc)
	h.mu.Unlock()

	h.sendToChannels(channels, event)
	return event, nil
}

func (h *liveEditSessionHub) UpdatePresence(target *liveEditTarget, clientID string, user string, selection *liveEditSelection) (liveEditEvent, error) {
	h.mu.Lock()
	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		h.mu.Unlock()
		return liveEditEvent{}, err
	}

	doc.LastActiveAt = time.Now()
	h.updatePresenceLocked(doc, clientID, user, selection)
	event := h.presenceEventLocked(doc, clientID, user)
	channels := h.subscriberChannelsLocked(doc)
	h.mu.Unlock()

	h.sendToChannels(channels, event)
	return event, nil
}

func (h *liveEditSessionHub) Leave(target *liveEditTarget, clientID string) liveEditEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	doc, ok := h.documents[target.FilePath]
	if !ok {
		return liveEditEvent{
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

func (h *liveEditSessionHub) unsubscribe(documentKey string, clientID string) {
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

func (h *liveEditSessionHub) watchDocument(documentKey string) {
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
		presenceChanged := h.pruneStalePresenceLocked(doc, time.Now())
		presenceEvent := h.presenceEventLocked(doc, "", "")
		presenceChannels := h.subscriberChannelsLocked(doc)
		h.mu.Unlock()

		if presenceChanged {
			h.sendToChannels(presenceChannels, presenceEvent)
		}

		info, err := os.Stat(filePath)
		if err != nil || !info.Mode().IsRegular() || !info.ModTime().After(lastDiskModTime) {
			continue
		}

		content, modTime, err := readLiveEditTextFile(filePath)
		if err != nil {
			continue
		}

		h.mu.Lock()
		doc, ok = h.documents[documentKey]
		if !ok {
			h.mu.Unlock()
			return
		}
		if content == doc.Content && !modTime.After(doc.LastDiskModTime) {
			h.mu.Unlock()
			continue
		}

		doc.Content = content
		doc.Version++
		doc.LastSavedAt = modTime
		doc.LastDiskModTime = modTime
		doc.LastActiveAt = time.Now()
		event := h.snapshotEventLocked(doc, "", "", true)
		channels := h.subscriberChannelsLocked(doc)
		h.mu.Unlock()

		h.sendToChannels(channels, event)
	}
}

func (h *liveEditSessionHub) ensureDocumentLocked(target *liveEditTarget) (*liveEditDocument, error) {
	if doc, ok := h.documents[target.FilePath]; ok {
		if err := h.refreshDocumentFromDiskLocked(doc); err != nil {
			return nil, err
		}
		return doc, nil
	}

	content, modTime, err := readLiveEditTextFile(target.FilePath)
	if err != nil {
		return nil, err
	}

	doc := &liveEditDocument{
		Key:             target.FilePath,
		FilePath:        target.FilePath,
		FileName:        target.FileName,
		DisplayPath:     target.DisplayPath,
		Mode:            target.AceMode,
		Content:         content,
		Version:         1,
		LastSavedAt:     modTime,
		LastDiskModTime: modTime,
		LastActiveAt:    time.Now(),
		Subscribers:     make(map[string]chan liveEditEvent),
		Presence:        make(map[string]liveEditPresence),
	}
	h.documents[target.FilePath] = doc
	return doc, nil
}

func (h *liveEditSessionHub) pruneStalePresenceLocked(doc *liveEditDocument, now time.Time) bool {
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

func (h *liveEditSessionHub) refreshDocumentFromDiskLocked(doc *liveEditDocument) error {
	info, err := os.Stat(doc.FilePath)
	if err != nil || !info.Mode().IsRegular() || !info.ModTime().After(doc.LastDiskModTime) {
		return err
	}

	content, modTime, err := readLiveEditTextFile(doc.FilePath)
	if err != nil {
		return err
	}
	if content != doc.Content {
		doc.Content = content
		doc.Version++
	}
	doc.LastSavedAt = modTime
	doc.LastDiskModTime = modTime
	return nil
}

func (h *liveEditSessionHub) updatePresenceLocked(doc *liveEditDocument, clientID string, user string, selection *liveEditSelection) {
	presence := doc.Presence[clientID]
	presence.ClientID = clientID
	presence.User = user
	presence.Color = liveEditPresenceColor(clientID)
	presence.Selection = selection
	presence.UpdatedAt = time.Now().UnixMilli()
	doc.Presence[clientID] = presence
}

func (h *liveEditSessionHub) snapshotEventLocked(doc *liveEditDocument, clientID string, user string, external bool) liveEditEvent {
	return liveEditEvent{
		Type:      "snapshot",
		ClientID:  clientID,
		User:      user,
		Version:   doc.Version,
		Content:   doc.Content,
		Mode:      doc.Mode,
		Path:      doc.DisplayPath,
		FileName:  doc.FileName,
		SavedAt:   doc.LastSavedAt.UnixMilli(),
		External:  external,
		Presence:  h.presenceListLocked(doc),
		Timestamp: time.Now().UnixMilli(),
	}
}

func (h *liveEditSessionHub) presenceEventLocked(doc *liveEditDocument, clientID string, user string) liveEditEvent {
	return liveEditEvent{
		Type:      "presence",
		ClientID:  clientID,
		User:      user,
		Version:   doc.Version,
		Presence:  h.presenceListLocked(doc),
		Timestamp: time.Now().UnixMilli(),
	}
}

func (h *liveEditSessionHub) presenceListLocked(doc *liveEditDocument) []liveEditPresence {
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

func (h *liveEditSessionHub) subscriberChannelsLocked(doc *liveEditDocument) []chan liveEditEvent {
	if doc == nil {
		return nil
	}
	channels := make([]chan liveEditEvent, 0, len(doc.Subscribers))
	for _, ch := range doc.Subscribers {
		channels = append(channels, ch)
	}
	return channels
}

func (h *liveEditSessionHub) sendToChannels(channels []chan liveEditEvent, event liveEditEvent) {
	for _, ch := range channels {
		h.enqueueEvent(ch, event)
	}
}

func (h *liveEditSessionHub) enqueueEvent(ch chan liveEditEvent, event liveEditEvent) {
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

func liveEditPresenceColor(clientID string) string {
	palette := []string{
		"#0f766e",
		"#b45309",
		"#1d4ed8",
		"#be123c",
		"#7c3aed",
		"#047857",
		"#c2410c",
		"#334155",
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(clientID))
	return palette[int(hasher.Sum32())%len(palette)]
}

func readLiveEditTextFile(filePath string) (string, time.Time, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", time.Time{}, err
	}
	if !info.Mode().IsRegular() {
		return "", time.Time{}, errors.New("only regular files can be opened in live edit")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", time.Time{}, err
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", time.Time{}, errLiveEditNonText
	}

	return string(data), info.ModTime(), nil
}
