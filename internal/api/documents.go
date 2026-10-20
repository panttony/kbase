package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func decodeDocumentRequest(w http.ResponseWriter, r *http.Request) (documentRequest, bool) {
	var req documentRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeError(w, invalidRequest("malformed JSON body"))
		return documentRequest{}, false
	}
	if req.Content == "" {
		writeError(w, invalidRequest("content must not be empty"))
		return documentRequest{}, false
	}
	return req, true
}

func (h *Handler) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeDocumentRequest(w, r)
	if !ok {
		return
	}
	p, err := process(req.Content)
	if err != nil {
		writeError(w, err)
		return
	}

	doc := fromParsed(p)
	stored, err := h.repo.Create(r.Context(), doc)
	if err != nil {
		writeError(w, mapRepoError(err))
		return
	}
	stored.PlainText = doc.PlainText
	if err := h.idx.Add(r.Context(), stored); err != nil {
		writeError(w, err)
		return
	}
	h.invalidateCache(r)

	writeJSON(w, http.StatusCreated, toDocumentResponse(stored))
}

// returns the current version of a document.
func (h *Handler) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	doc, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeError(w, mapRepoError(err))
		return
	}
	writeJSON(w, http.StatusOK, toDocumentResponse(doc))
}

// replaces a document's body and metadata
func (h *Handler) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeDocumentRequest(w, r)
	if !ok {
		return
	}
	p, err := process(req.Content)
	if err != nil {
		writeError(w, err)
		return
	}

	doc := fromParsed(p)
	doc.ID = id
	stored, err := h.repo.Update(r.Context(), doc)
	if err != nil {
		writeError(w, mapRepoError(err))
		return
	}
	stored.PlainText = doc.PlainText
	if err := h.idx.Add(r.Context(), stored); err != nil {
		writeError(w, err)
		return
	}
	h.invalidateCache(r)

	writeJSON(w, http.StatusOK, toDocumentResponse(stored))
}

// soft-deletes a document, removes it from the index and
func (h *Handler) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeError(w, mapRepoError(err))
		return
	}
	if err := h.idx.Delete(r.Context(), id.String()); err != nil {
		writeError(w, err)
		return
	}
	h.invalidateCache(r)

	w.WriteHeader(http.StatusNoContent)
}

// returns the version history (oldest first) of a document
func (h *Handler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	versions, err := h.repo.ListVersions(r.Context(), id)
	if err != nil {
		writeError(w, mapRepoError(err))
		return
	}
	out := make([]versionResponse, 0, len(versions))
	for _, v := range versions {
		out = append(out, toVersionResponse(v))
	}
	writeJSON(w, http.StatusOK, out)
}

// returns exactly one version by document id and number
func (h *Handler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var number int
	if n, err := strconv.Atoi(r.PathValue("number")); err != nil || n < 1 {
		writeError(w, invalidRequest("invalid version number"))
		return
	} else {
		number = n
	}
	v, err := h.repo.GetVersion(r.Context(), id, number)
	if err != nil {
		writeError(w, mapRepoError(err))
		return
	}
	writeJSON(w, http.StatusOK, toVersionResponse(v))
}

// drops all cached search keys after a document mutation so
func (h *Handler) invalidateCache(r *http.Request) {
	if h.cache == nil {
		return
	}
	_ = h.cache.InvalidateAll(r.Context(), "search:v1:")
}
