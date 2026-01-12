package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GoogleBooksHandler struct {
	Client *http.Client
}

func NewGoogleBooksHandler() *GoogleBooksHandler {
	return &GoogleBooksHandler{
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

type GoogleBookDTO struct {
	ID            string   `json:"id"` 
	Title         string   `json:"title"`
	Authors       []string `json:"authors"`
	Author        string   `json:"author"` 
	CoverURL      string   `json:"coverUrl"`
	Description   string   `json:"description"`
	Categories    []string `json:"categories"`
	PublishedYear int      `json:"publishedYear"`
	PageCount     int      `json:"pageCount"`
	Maturity      string   `json:"maturity"` 
}


func (h *GoogleBooksHandler) Search(w http.ResponseWriter, r *http.Request) {
	enableCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "missing query param: q", http.StatusBadRequest)
		return
	}

	max := r.URL.Query().Get("max")
	if max == "" {
		max = "12"
	}

	u := "https://www.googleapis.com/books/v1/volumes?q=" + url.QueryEscape(q) + "&maxResults=" + url.QueryEscape(max)

	items, err := h.fetchVolumes(r, u)
	if err != nil {
		http.Error(w, "google books error: "+err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, items)
}

func (h *GoogleBooksHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	enableCORS(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/books/google/")
	id = strings.TrimSpace(id)
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	u := "https://www.googleapis.com/books/v1/volumes/" + url.PathEscape(id)

	dto, err := h.fetchVolumeByID(r, u)
	if err != nil {
		http.Error(w, "google books error: "+err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, dto)
}

type gbSearchResp struct {
	Items []gbItem `json:"items"`
}
type gbItem struct {
	ID         string         `json:"id"`
	VolumeInfo gbVolumeInfo   `json:"volumeInfo"`
}
type gbVolumeInfo struct {
	Title         string   `json:"title"`
	Authors       []string `json:"authors"`
	Description   string   `json:"description"`
	Categories    []string `json:"categories"`
	PublishedDate string   `json:"publishedDate"` 
	PageCount     int      `json:"pageCount"`
	Maturity      string   `json:"maturityRating"`
	ImageLinks    struct {
		Thumbnail string `json:"thumbnail"`
		Small     string `json:"smallThumbnail"`
	} `json:"imageLinks"`
}

func (h *GoogleBooksHandler) fetchVolumes(r *http.Request, u string) ([]GoogleBookDTO, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var raw gbSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	out := make([]GoogleBookDTO, 0, len(raw.Items))
	for _, it := range raw.Items {
		out = append(out, mapToDTO(it))
	}
	return out, nil
}

func (h *GoogleBooksHandler) fetchVolumeByID(r *http.Request, u string) (*GoogleBookDTO, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var it gbItem
	if err := json.NewDecoder(resp.Body).Decode(&it); err != nil {
		return nil, err
	}

	dto := mapToDTO(it)
	return &dto, nil
}

func mapToDTO(it gbItem) GoogleBookDTO {
	vi := it.VolumeInfo

	author := ""
	if len(vi.Authors) > 0 {
		author = strings.Join(vi.Authors, ", ")
	}

	cover := vi.ImageLinks.Thumbnail
	if cover == "" {
		cover = vi.ImageLinks.Small
	}
	cover = strings.ReplaceAll(cover, "http://", "https://")

	return GoogleBookDTO{
		ID:            it.ID,
		Title:         vi.Title,
		Authors:       vi.Authors,
		Author:        author,
		CoverURL:      cover,
		Description:   vi.Description,
		Categories:    vi.Categories,
		PublishedYear: parseYear(vi.PublishedDate),
		PageCount:     vi.PageCount,
		Maturity:      vi.Maturity,
	}
}

func parseYear(s string) int {
	if len(s) >= 4 {
		yearPart := s[:4]
		var y int
		_, _ = fmt.Sscanf(yearPart, "%d", &y)
		return y
	}
	return 0
}


func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func enableCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}
