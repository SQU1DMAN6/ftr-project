package repository

import (
	"errors"
	repoStore "inkdrop/repository"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const documentEditMaxFileSize int64 = 25 * 1024 * 1024

var documentEditSessions = newDocumentEditSessionHub()

type documentEditEvent struct {
	Type            string                  `json:"type"`
	ClientID        string                  `json:"clientId,omitempty"`
	User            string                  `json:"user,omitempty"`
	Version         int64                   `json:"version,omitempty"`
	LiveVersion     int64                   `json:"liveVersion,omitempty"`
	BaseLiveVersion int64                   `json:"baseLiveVersion,omitempty"`
	Path            string                  `json:"path,omitempty"`
	FileName        string                  `json:"fileName,omitempty"`
	FileURL         string                  `json:"fileUrl,omitempty"`
	DocumentHTML    string                  `json:"documentHtml,omitempty"`
	Operations      []documentEditOperation `json:"operations,omitempty"`
	SavedAt         int64                   `json:"savedAt,omitempty"`
	External        bool                    `json:"external,omitempty"`
	Presence        []liveEditPresence      `json:"presence,omitempty"`
	Message         string                  `json:"message,omitempty"`
	Timestamp       int64                   `json:"timestamp"`
}

type documentEditOperation struct {
	Kind string `json:"kind,omitempty"`
	From int    `json:"from,omitempty"`
	To   int    `json:"to,omitempty"`
	Text string `json:"text,omitempty"`
}

type documentEditOperationBatch struct {
	BaseLiveVersion int64                   `json:"baseLiveVersion"`
	LiveVersion     int64                   `json:"liveVersion"`
	Timestamp       int64                   `json:"timestamp"`
	Operations      []documentEditOperation `json:"operations,omitempty"`
}

type documentEditDocument struct {
	Key              string
	FilePath         string
	FileName         string
	DisplayPath      string
	SourceURL        string
	Version          int64
	LiveVersion      int64
	DocumentHTML     string
	LastSavedAt      time.Time
	LastDiskModTime  time.Time
	LastDiskSize     int64
	LastActiveAt     time.Time
	OperationHistory []documentEditOperationBatch
	Subscribers      map[string]chan documentEditEvent
	Presence         map[string]liveEditPresence
	WatcherRunning   bool
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

func sanitizeDocumentEditOperations(operations []documentEditOperation) []documentEditOperation {
	if len(operations) == 0 {
		return nil
	}

	sanitized := make([]documentEditOperation, 0, min(len(operations), 64))
	for _, operation := range operations {
		kind := strings.ToLower(strings.TrimSpace(operation.Kind))
		switch kind {
		case "replace_text":
			from := operation.From
			to := operation.To
			if from < 1 {
				from = 1
			}
			if to < from {
				to = from
			}
			if from == to && operation.Text == "" {
				continue
			}
			sanitized = append(sanitized, documentEditOperation{
				Kind: kind,
				From: from,
				To:   to,
				Text: operation.Text,
			})
		case "delete_range":
			from := operation.From
			to := operation.To
			if from < 1 {
				from = 1
			}
			if to < from {
				to = from
			}
			if from == to {
				continue
			}
			sanitized = append(sanitized, documentEditOperation{
				Kind: kind,
				From: from,
				To:   to,
			})
		}
		if len(sanitized) >= 64 {
			break
		}
	}

	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func cloneDocumentEditOperations(operations []documentEditOperation) []documentEditOperation {
	if len(operations) == 0 {
		return nil
	}
	cloned := make([]documentEditOperation, len(operations))
	copy(cloned, operations)
	return cloned
}

func documentEditTextLengthUnits(value string) int {
	return len(utf16.Encode([]rune(value)))
}

func transformDocumentEditPosition(pos int, prior documentEditOperation, stickToEnd bool) int {
	if pos < 1 {
		pos = 1
	}
	start := max(1, prior.From)
	end := max(start, prior.To)
	inserted := 0
	if prior.Kind == "replace_text" {
		inserted = documentEditTextLengthUnits(prior.Text)
	}
	deleted := end - start
	delta := inserted - deleted

	if deleted == 0 {
		if pos < start {
			return pos
		}
		if pos > start {
			return pos + delta
		}
		if stickToEnd {
			return pos + inserted
		}
		return pos
	}

	if pos < start {
		return pos
	}
	if pos > end {
		return pos + delta
	}
	if stickToEnd {
		return start + inserted
	}
	return start
}

func transformDocumentEditOperationAgainst(operation documentEditOperation, prior documentEditOperation) documentEditOperation {
	switch operation.Kind {
	case "replace_text", "delete_range":
		operation.From = transformDocumentEditPosition(
			operation.From,
			prior,
			false,
		)
		operation.To = transformDocumentEditPosition(
			operation.To,
			prior,
			true,
		)
		if operation.To < operation.From {
			operation.To = operation.From
		}
	}
	return operation
}

func transformDocumentEditOperations(operations []documentEditOperation, priorBatches []documentEditOperationBatch) []documentEditOperation {
	if len(operations) == 0 || len(priorBatches) == 0 {
		return cloneDocumentEditOperations(operations)
	}
	transformed := cloneDocumentEditOperations(operations)
	for _, batch := range priorBatches {
		for _, prior := range batch.Operations {
			for index := range transformed {
				transformed[index] = transformDocumentEditOperationAgainst(
					transformed[index],
					prior,
				)
			}
		}
	}
	return sanitizeDocumentEditOperations(transformed)
}

func cloneDocumentEditOperationBatches(batches []documentEditOperationBatch) []documentEditOperationBatch {
	if len(batches) == 0 {
		return nil
	}
	cloned := make([]documentEditOperationBatch, 0, len(batches))
	for _, batch := range batches {
		cloned = append(cloned, documentEditOperationBatch{
			BaseLiveVersion: batch.BaseLiveVersion,
			LiveVersion:     batch.LiveVersion,
			Timestamp:       batch.Timestamp,
			Operations:      cloneDocumentEditOperations(batch.Operations),
		})
	}
	return cloned
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

func (h *documentEditSessionHub) UpdateContent(target *liveEditTarget, clientID string, user string, data []byte, documentHTML string, selection *liveEditSelection) (documentEditEvent, error) {
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
	if documentHTML != "" {
		doc.DocumentHTML = documentHTML
	}

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

func (h *documentEditSessionHub) UpdateStage(target *liveEditTarget, clientID string, user string, baseLiveVersion int64, documentHTML string, operations []documentEditOperation, selection *liveEditSelection) (documentEditEvent, error) {
	operations = sanitizeDocumentEditOperations(operations)
	if documentHTML == "" {
		if len(operations) == 0 {
			return documentEditEvent{}, errors.New("documentHtml or operations are required")
		}
	}

	h.mu.Lock()
	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		h.mu.Unlock()
		return documentEditEvent{}, err
	}

	doc.LastActiveAt = time.Now()
	h.updatePresenceLocked(doc, clientID, user, selection)
	if documentHTML != "" {
		doc.DocumentHTML = documentHTML
	}

	stageBaseLiveVersion := doc.LiveVersion
	transformedOperations := operations
	if len(operations) > 0 && baseLiveVersion < doc.LiveVersion {
		priorBatches := h.operationBatchesSinceLocked(doc, baseLiveVersion)
		if len(priorBatches) == 0 && baseLiveVersion != doc.LiveVersion {
			transformedOperations = nil
		} else {
			transformedOperations = transformDocumentEditOperations(
				operations,
				priorBatches,
			)
		}
	}
	doc.LiveVersion++
	if len(transformedOperations) > 0 {
		doc.OperationHistory = append(
			doc.OperationHistory,
			documentEditOperationBatch{
				BaseLiveVersion: stageBaseLiveVersion,
				LiveVersion:     doc.LiveVersion,
				Timestamp:       time.Now().UnixMilli(),
				Operations:      cloneDocumentEditOperations(transformedOperations),
			},
		)
		if len(doc.OperationHistory) > 512 {
			doc.OperationHistory = append(
				[]documentEditOperationBatch(nil),
				doc.OperationHistory[len(doc.OperationHistory)-512:]...,
			)
		}
	}
	event := h.stageEventLocked(doc, clientID, user, stageBaseLiveVersion, transformedOperations)
	channels := h.subscriberChannelsLocked(doc)
	h.mu.Unlock()

	h.sendToChannels(channels, event)
	return event, nil
}

func (h *documentEditSessionHub) OperationBatchesSince(target *liveEditTarget, fromLiveVersion int64) []documentEditOperationBatch {
	h.mu.Lock()
	defer h.mu.Unlock()

	doc, err := h.ensureDocumentLocked(target)
	if err != nil {
		return nil
	}

	return h.operationBatchesSinceLocked(doc, fromLiveVersion)
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
		doc.LiveVersion++
		doc.DocumentHTML = ""
		doc.OperationHistory = nil
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
		Key:              target.FilePath,
		FilePath:         target.FilePath,
		FileName:         target.FileName,
		DisplayPath:      target.DisplayPath,
		SourceURL:        target.SourceURL,
		Version:          1,
		LiveVersion:      1,
		LastSavedAt:      info.ModTime(),
		LastDiskModTime:  info.ModTime(),
		LastDiskSize:     info.Size(),
		LastActiveAt:     time.Now(),
		OperationHistory: nil,
		Subscribers:      make(map[string]chan documentEditEvent),
		Presence:         make(map[string]liveEditPresence),
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
	doc.LiveVersion++
	doc.DocumentHTML = ""
	doc.OperationHistory = nil
	doc.LastSavedAt = info.ModTime()
	doc.LastDiskModTime = info.ModTime()
	doc.LastDiskSize = info.Size()
	return nil
}

func (h *documentEditSessionHub) operationBatchesSinceLocked(doc *documentEditDocument, fromLiveVersion int64) []documentEditOperationBatch {
	if doc == nil || len(doc.OperationHistory) == 0 {
		return nil
	}

	current := fromLiveVersion
	result := make([]documentEditOperationBatch, 0)
	for _, batch := range doc.OperationHistory {
		if batch.LiveVersion <= fromLiveVersion {
			continue
		}
		if batch.BaseLiveVersion != current {
			return nil
		}
		result = append(result, batch)
		current = batch.LiveVersion
	}
	if current != doc.LiveVersion {
		return nil
	}
	return cloneDocumentEditOperationBatches(result)
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
		Type:         "snapshot",
		ClientID:     clientID,
		User:         user,
		Version:      doc.Version,
		LiveVersion:  doc.LiveVersion,
		Path:         doc.DisplayPath,
		FileName:     doc.FileName,
		FileURL:      doc.SourceURL,
		DocumentHTML: doc.DocumentHTML,
		SavedAt:      doc.LastSavedAt.UnixMilli(),
		External:     external,
		Presence:     h.presenceListLocked(doc),
		Timestamp:    time.Now().UnixMilli(),
	}
}

func (h *documentEditSessionHub) presenceEventLocked(doc *documentEditDocument, clientID string, user string) documentEditEvent {
	return documentEditEvent{
		Type:        "presence",
		ClientID:    clientID,
		User:        user,
		Version:     doc.Version,
		LiveVersion: doc.LiveVersion,
		Presence:    h.presenceListLocked(doc),
		Timestamp:   time.Now().UnixMilli(),
	}
}

func (h *documentEditSessionHub) stageEventLocked(doc *documentEditDocument, clientID string, user string, baseLiveVersion int64, operations []documentEditOperation) documentEditEvent {
	return documentEditEvent{
		Type:            "stage",
		ClientID:        clientID,
		User:            user,
		Version:         doc.Version,
		LiveVersion:     doc.LiveVersion,
		BaseLiveVersion: baseLiveVersion,
		Path:            doc.DisplayPath,
		FileName:        doc.FileName,
		FileURL:         doc.SourceURL,
		DocumentHTML:    doc.DocumentHTML,
		Operations:      cloneDocumentEditOperations(operations),
		SavedAt:         doc.LastSavedAt.UnixMilli(),
		Presence:        h.presenceListLocked(doc),
		Timestamp:       time.Now().UnixMilli(),
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
