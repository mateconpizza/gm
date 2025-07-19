//go:build ignore

// Package cmd contains the web server implementation.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mateconpizza/gm/internal/bookmark/scraper"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	record "github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type FetchDataResponse struct {
	Title      string   `json:"title"`
	Desc       string   `json:"desc"`
	Tags       []string `json:"tags"`
	FaviconURL string   `json:"favicon"`
}

type OptFn func(*Options)

type Options struct {
	DB           *db.SQLite
	addr         string
	templatePath string
}

type Server struct {
	*http.Server
	*Options
	isActive     int32
	records      []*bookmark.Bookmark
	itemsPerPage int
	tmpl         *template.Template
}

type ParamsBuilder struct {
	params RequestParams
}

func (b *ParamsBuilder) Query(q string) *ParamsBuilder {
	b.params.Query = q
	return b
}

func (b *ParamsBuilder) Filter(filter string) *ParamsBuilder {
	b.params.FilterBy = filter
	return b
}

func (b *ParamsBuilder) Tag(tag string) *ParamsBuilder {
	b.params.Tag = tag
	return b
}

func (b *ParamsBuilder) Page(page int) *ParamsBuilder {
	b.params.Page = page
	return b
}

func (b *ParamsBuilder) Debug(debug bool) *ParamsBuilder {
	b.params.Debug = debug
	return b
}

func (b *ParamsBuilder) Favorite(favorite bool) *ParamsBuilder {
	b.params.Favorites = favorite
	return b
}

func (b *ParamsBuilder) Databases(databases []string) *ParamsBuilder {
	b.params.Databases = databases
	return b
}

func (b *ParamsBuilder) Letter(letter string) *ParamsBuilder {
	b.params.Letter = letter
	return b
}

func (b *ParamsBuilder) Database(database string) *ParamsBuilder {
	b.params.CurrentDB = database
	return b
}

func (b *ParamsBuilder) Edit(bID int) *ParamsBuilder {
	b.params.EditID = bID
	return b
}

func (b *ParamsBuilder) Build(path string) string {
	return b.params.PaginationURL(path, b.params.Page)
}

func (b *ParamsBuilder) BuildParams() *RequestParams {
	c := b.params
	return &c
}

// RequestParams holds the query parameters from the request.
type RequestParams struct {
	CurrentDB  string
	Databases  []string
	Debug      bool
	Favorites  bool
	FilterBy   string
	Letter     string
	EditID     int
	Page       int
	Query      string
	Tag        string
	TagAutoCmp string
}

func (p *RequestParams) With() *ParamsBuilder {
	return &ParamsBuilder{params: *p}
}

func (p *RequestParams) queryValues() url.Values {
	q := url.Values{}

	if p.Query != "" {
		q.Set("q", p.Query)
	}
	if p.Tag != "" {
		q.Set("tag", p.Tag)
	}
	if p.FilterBy != "" {
		q.Set("filter", p.FilterBy)
	}
	if p.Favorites {
		q.Set("favorites", "true")
	}
	if p.CurrentDB != "" {
		q.Set("db", p.CurrentDB)
	}
	if p.Letter != "" {
		q.Set("letter", p.Letter)
	}
	if p.Debug {
		q.Set("debug", "1")
	}

	return q
}

// PaginationURL returns a URL with preserved filters and the given page number.
func (p *RequestParams) PaginationURL(path string, page int) string {
	q := p.queryValues()
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}

	return path + "?" + q.Encode()
}

func (p *RequestParams) IsFilterActive(name string) bool {
	return p.FilterBy == name
}

// BaseURL constructs the base URL for pagination, preserving query/tag/filter.
func (p *RequestParams) BaseURL(path string) string {
	return path + "?" + p.queryValues().Encode()
}

// PaginationInfo holds pagination-related data.
type PaginationInfo struct {
	CurrentPage    int
	TotalPages     int
	ItemsPerPage   int
	TotalBookmarks int
	StartIndex     int
	EndIndex       int
}

// TemplateData holds all data needed for template rendering.
type TemplateData struct {
	App            *config.AppConfig
	Bookmarks      []*bookmark.Bookmark
	Bookmark       *bookmark.Bookmark
	Pagination     PaginationInfo
	Params         *RequestParams
	TagGroups      map[string][]string
	CurrentPath    string
	BaseURL        string
	NewestURL      string
	OldestURL      string
	LastVisitedURL string
	FavoritesURL   string
	MoreVisitsURL  string
	ClearTagURL    string
	ClearQueryURL  string
	DebugToggleURL string
}

func WithTemplatePath(path string) OptFn {
	return func(o *Options) {
		o.templatePath = path
	}
}

func WithDB(r *db.SQLite) OptFn {
	return func(o *Options) {
		o.DB = r
	}
}

func WithAddr(addr string) OptFn {
	return func(o *Options) {
		o.addr = addr
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	if !atomic.CompareAndSwapInt32(&s.isActive, 0, 1) {
		return ErrServerAlreadyRunning
	}

	return s.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.isActive, 0, 1) {
		log.Println("Server is not running")
		return nil
	}

	return s.Server.Shutdown(ctx)
}

// LoadRecords loads the database bookmarks.
func (s *Server) LoadRecords() error {
	var err error
	s.records, err = s.DB.AllPtr()
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) LoadDB(p string) error {
	r, err := db.New(filepath.Join(config.App.Path.Data, p))
	if err != nil {
		return err
	}

	s.DB = r

	return nil
}

func (s *Server) Records(w http.ResponseWriter, r *http.Request) {
	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.LoadRecords(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create working copy of bookmarks
	currentBookmarks := make([]*bookmark.Bookmark, len(s.records))
	copy(currentBookmarks, s.records)

	// Apply filters and sorting
	filteredAndSortedBookmarks := applyFiltersAndSorting(currentBookmarks, p)

	// Calculate pagination
	pagination := calculatePagination(len(filteredAndSortedBookmarks), p.Page, s.itemsPerPage)

	// Get paginated bookmarks
	paginatedBookmarks := paginateBookmarks(filteredAndSortedBookmarks, pagination)

	// Get tags function
	tagsFunc := getTagsFn(p, s.records, paginatedBookmarks)

	// Log debug information
	logDebugInfo(p, pagination)

	// Build template data
	data := buildTemplateData(r, paginatedBookmarks, p, p.BaseURL(r.URL.Path), tagsFunc, pagination)

	// Execute template
	if err := s.tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) setItemsPerPage(items int) {
	s.itemsPerPage = items
}

func (s *Server) Update(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		slog.Error("updating bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	bj := bookmark.NewJSON()
	if err := json.NewDecoder(r.Body).Decode(bj); err != nil {
		slog.Error("updating bookmark", "error", err)
		encodeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("updating bookmark: closing request body", "error", err)
		}
	}()

	newB := bookmark.NewFromJSON(bj)
	oldB, err := s.DB.ByID(bj.ID)
	if err != nil {
		slog.Error("updating bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if bytes.Equal(newB.Buffer(), oldB.Buffer()) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no changes found"})
		return
	}

	newB.CreatedAt = oldB.CreatedAt
	newB.LastVisit = oldB.LastVisit
	newB.Favorite = oldB.Favorite
	newB.VisitCount = oldB.VisitCount
	if oldB.URL == newB.URL {
		newB.FaviconURL = oldB.FaviconURL
	}

	if err := s.DB.Update(context.Background(), newB, oldB); err != nil {
		slog.Error("updating bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).
		Encode(map[string]string{"message": "Bookmark updated successfully!", "id": strconv.Itoa(bj.ID)})
}

// ToggleFavorite toggles the favorite status of a bookmark.
func (s *Server) ToggleFavorite(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// idStr := r.URL.Query().Get("id")
	idStr := r.PathValue("id")
	bID, err := strconv.Atoi(idStr)
	if err != nil || bID < 1 {
		slog.Error("toggle favorite", "error", err)
		encodeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		slog.Error("updating bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	b, err := s.DB.ByID(bID)
	if err != nil {
		slog.Error("toggle favorite", "error", err, "id", bID)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	mesg := "Bookmark " + txt.Shorten(b.URL, 60)
	if b.Favorite {
		mesg += " : Unfavorited"
	} else {
		mesg += " : Favorited"
	}
	b.Favorite = !b.Favorite

	if err := s.DB.SetFavorite(context.Background(), b); err != nil {
		slog.Error("toggle favorite", "error", err, "id", bID)
		encodeErr(w, http.StatusInternalServerError, err.Error())
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).
		Encode(map[string]string{"message": mesg})
}

func (s *Server) Delete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		slog.Error("deleting bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		err := db.ErrRecordIDNotProvided
		slog.Error("deleting bookmark", "error", err)
		encodeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	bID, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Error("deleting bookmark", "error", err)
		encodeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	b, err := s.DB.ByID(bID)
	if err != nil {
		slog.Error("deleting bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := db.RemoveReorder(context.Background(), s.DB, []*bookmark.Bookmark{b}); err != nil {
		slog.Error("deleting bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Bookmark deleted successfully!"})
}

func (s *Server) EditByID(w http.ResponseWriter, r *http.Request) {
	fmt.Println("handlerEditNew called...")
	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "Bookmark ID not provided.", http.StatusBadRequest)
		return
	}

	bID, err := strconv.Atoi(idStr)
	if err != nil {
		if errors.Is(err, strconv.ErrSyntax) {
			http.Error(w, bookmark.ErrInvalidID.Error()+" '"+idStr+"'", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	b, err := s.DB.ByID(bID)
	if err != nil {
		http.Error(w, "Failed to retrieve bookmark data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := &TemplateData{Params: p, Bookmark: b}

	if err := s.tmpl.ExecuteTemplate(w, "edit-bookmark", data); err != nil {
		http.Error(w, "Failed to render bookmark edit page: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) fetchData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	u := r.URL.Query().Get("url")
	if u == "" {
		slog.Debug("fetching URL", "error", "empty URL")
		return
	}

	slog.Debug("fetching URL", "url", u)

	sc := scraper.New(u)
	if err := sc.Start(); err != nil {
		slog.Error("scrape new bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	b := bookmark.NewJSON()
	b.Title, _ = sc.Title()
	b.Desc, _ = sc.Desc()
	k, _ := sc.Keywords()
	b.Tags = strings.Split(k, ",")
	b.FaviconURL, _ = sc.Favicon()

	w.WriteHeader(http.StatusOK)

	responsePayload := &FetchDataResponse{
		Title:      b.Title,
		Desc:       b.Desc,
		Tags:       b.Tags,
		FaviconURL: b.FaviconURL,
	}

	err := json.NewEncoder(w).Encode(responsePayload)
	if err != nil {
		slog.Error("fetching response: failed to encode JSON", "error", err, "url", u)
		encodeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *Server) someTesting(w http.ResponseWriter, r *http.Request) {
	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := &TemplateData{Params: p}

	if err := s.tmpl.ExecuteTemplate(w, "bookmark-new", data); err != nil {
		http.Error(w, "Failed to render bookmark edit page: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) RenderPageCreate(w http.ResponseWriter, r *http.Request) {
	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := &TemplateData{Params: p}

	if err := s.tmpl.ExecuteTemplate(w, "bookmark-new", data); err != nil {
		http.Error(w, "Failed to render bookmark edit page: "+err.Error(), http.StatusInternalServerError)
	}
}

// ListTags serves the tags and count.
func (s *Server) ListTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	tags, err := db.TagsCounter(s.DB)
	if err != nil {
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(tags); err != nil {
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func (s *Server) NewRecord(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	p := parseRequestParams(r)
	if err := s.LoadDB(p.CurrentDB); err != nil {
		slog.Error("creating bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	bj := bookmark.NewJSON()
	if err := json.NewDecoder(r.Body).Decode(bj); err != nil {
		slog.Error("creating bookmark", "error", err)
		encodeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			slog.Error("creating bookmark: closing request body", "error", err)
		}
	}()

	if _, exists := s.DB.Has(bj.URL); exists {
		slog.Error("creating bookmark", "error", db.ErrRecordDuplicate)
		encodeErr(w, http.StatusBadRequest, db.ErrRecordDuplicate.Error())
		return
	}

	newB := record.NewFromJSON(bj)
	if err := record.Validate(newB); err != nil {
		slog.Error("creating bookmark", "error", err)
		encodeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.DB.InsertOne(context.Background(), newB); err != nil {
		slog.Error("creating bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	b, err := s.DB.ByID(s.DB.MaxID())
	if err != nil {
		slog.Error("creating bookmark", "error", err)
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).
		Encode(map[string]string{"message": fmt.Sprintf("New bookmark created with ID %d", b.ID)})
}

func (s *Server) ListDBs(w http.ResponseWriter, r *http.Request) {
	paths, err := db.List(s.DB.Cfg.Path)
	if err != nil {
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	dbs := make([]string, 0, len(paths))
	for i := range paths {
		dbs = append(dbs, filepath.Base(paths[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(dbs); err != nil {
		encodeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func newServer(h http.Handler, opts ...OptFn) (*Server, error) {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}

	if o.templatePath == "" {
		return nil, ErrTmplPathNotFound
	}
	tmpl, err := createMainTemplate(o.templatePath)
	if err != nil {
		log.Fatal(err)
	}

	if o.DB == nil {
		return nil, ErrDBIsRequired
	}

	return &Server{
		Server: &http.Server{
			Addr:         o.addr,
			Handler:      h,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Options: o,
		tmpl:    tmpl,
	}, nil
}

// parseRequestParams extracts and validates query parameters from the request.
func parseRequestParams(r *http.Request) *RequestParams {
	q := r.URL.Query()

	debug := q.Get("debug") == "1"
	filterBy := q.Get("filter")
	letter := q.Get("letter")
	queryStr := q.Get("q")
	selectedDB := q.Get("db")
	tag := q.Get("tag")

	// tag selected from dropdown autocmp
	if tag == "" && strings.HasPrefix(queryStr, "#") {
		tag = queryStr
		queryStr = ""
	}

	dbList, err := db.List(config.App.Path.Data)
	if err != nil {
		slog.Error("error fetching available databases", "error", err)
		dbList = []string{config.App.DBName} // fallback
	}

	availableDBs := make([]string, 0, len(dbList))
	for _, path := range dbList {
		availableDBs = append(availableDBs, filepath.Base(path))
	}

	// Determine the active db
	if selectedDB == "" && len(availableDBs) > 0 {
		selectedDB = config.App.DBName
	}

	// parse page
	currentPage, err := strconv.Atoi(q.Get("page"))
	if err != nil || currentPage < 1 {
		currentPage = 1
	}

	pb := &ParamsBuilder{}
	return pb.
		Tag(tag).
		Query(queryStr).
		Filter(filterBy).
		Letter(letter).
		Database(selectedDB).
		Page(currentPage).
		Debug(debug).
		Databases(availableDBs).
		BuildParams()
}

func filterToggleURL(p *RequestParams, current, path string) string {
	if p.FilterBy == current {
		return p.With().Filter("").Build(path)
	}
	return p.With().Filter(current).Build(path)
}

// filterBookmarksByTag filters bookmarks by a specific tag.
func filterBookmarksByTag(bookmarks []*bookmark.Bookmark, tag string) []*bookmark.Bookmark {
	if tag == "" {
		return bookmarks
	}

	var filtered []*bookmark.Bookmark
	normalizedTag := strings.ToLower(strings.TrimPrefix(tag, "#"))

	for _, b := range bookmarks {
		for t := range strings.SplitSeq(b.Tags, ",") {
			if strings.EqualFold(strings.TrimSpace(t), normalizedTag) {
				filtered = append(filtered, b)
				break
			}
		}
	}

	return filtered
}

// filterBookmarksByQuery filters bookmarks by search query.
func filterBookmarksByQuery(bookmarks []*bookmark.Bookmark, query string) []*bookmark.Bookmark {
	if query == "" {
		return bookmarks
	}

	var filtered []*bookmark.Bookmark
	queryWords := strings.Fields(strings.ToLower(query))

	for _, b := range bookmarks {
		text := strings.ToLower(b.Title + " " + b.URL + " " + b.Desc + " " + b.Tags)
		matched := true

		for _, word := range queryWords {
			if !strings.Contains(text, word) {
				matched = false
				break
			}
		}

		if matched {
			filtered = append(filtered, b)
		}
	}

	return filtered
}

func filterBookmarksByLetter(bs []*bookmark.Bookmark, letter string) []*bookmark.Bookmark {
	if letter == "" {
		return bs
	}

	f := make([]*bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		b := bs[i]
		t := strings.Split(b.Tags, ",")
		if !strings.EqualFold(string(t[0][0]), letter) {
			continue
		}

		f = append(f, b)
	}

	return f
}

func groupTagsByLetter(tags []string) map[string][]string {
	sort.Strings(tags)
	grouped := make(map[string][]string)
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		first := strings.ToUpper(string(tag[0]))
		grouped[first] = append(grouped[first], "#"+tag)
	}
	return grouped
}

// applyFiltersAndSorting applies all filters and sorting to bookmarks.
func applyFiltersAndSorting(bs []*bookmark.Bookmark, p *RequestParams) []*bookmark.Bookmark {
	// Apply tag filter
	filtered := filterBookmarksByTag(bs, p.Tag)

	// Apply query filter
	filtered = filterBookmarksByQuery(filtered, p.Query)

	// Apply letter filter
	filtered = filterBookmarksByLetter(filtered, p.Letter)

	// Apply sorting (assuming filterBy is your sorting function)
	return sortBy(p.FilterBy, filtered)
}

// calculatePagination calculates pagination information.
func calculatePagination(totalBookmarks, currentPage, itemsPerPage int) PaginationInfo {
	totalPages := (totalBookmarks + itemsPerPage - 1) / itemsPerPage

	// Adjust currentPage if it's out of bounds
	if currentPage > totalPages && totalPages > 0 {
		currentPage = totalPages
	} else if totalPages == 0 {
		currentPage = 1
	}

	startIndex := min(max((currentPage-1)*itemsPerPage, 0), totalBookmarks)
	endIndex := min(startIndex+itemsPerPage, totalBookmarks)

	return PaginationInfo{
		CurrentPage:    currentPage,
		TotalPages:     totalPages,
		ItemsPerPage:   itemsPerPage,
		TotalBookmarks: totalBookmarks,
		StartIndex:     startIndex,
		EndIndex:       endIndex,
	}
}

// paginateBookmarks returns a slice of bookmarks for the current page.
func paginateBookmarks(bs []*bookmark.Bookmark, pag PaginationInfo) []*bookmark.Bookmark {
	return bs[pag.StartIndex:pag.EndIndex]
}

// getTagsFn returns appropriate tags extraction function based on filters.
func getTagsFn(p *RequestParams, bs, paginatedBs []*bookmark.Bookmark) func() []string {
	items := paginatedBs
	if p.Tag == "" && p.Query == "" {
		items = bs
	}

	return func() []string {
		return extractTags(items)
	}
}

// logDebugInfo logs debug information if debug mode is enabled.
func logDebugInfo(p *RequestParams, pagination PaginationInfo) {
	if !p.Debug {
		return
	}

	fmt.Println("Value of 'tag':", p.Tag)
	fmt.Println("Value of 'query':", p.Query)
	fmt.Println("Value of 'filterBy':", p.FilterBy)
	fmt.Println("Value of 'SelectedDatabase", p.CurrentDB)
	fmt.Printf("p.AvailableDatabases: %v\n", p.Databases)
	fmt.Printf("p.Letter: %v\n", p.Letter)

	fmt.Printf("DEBUG PAGINATION: itemsPerPage = %d\n", pagination.ItemsPerPage)
	fmt.Printf("DEBUG PAGINATION: totalBookmarks = %d\n", pagination.TotalBookmarks)
	fmt.Printf("DEBUG PAGINATION: currentPage = %d\n", pagination.CurrentPage)
	fmt.Printf("DEBUG PAGINATION: totalPages = %d\n", pagination.TotalPages)
	fmt.Printf("DEBUG PAGINATION: startIndex = %d\n", pagination.StartIndex)
	fmt.Printf("DEBUG PAGINATION: endIndex = %d\n", pagination.EndIndex)
	fmt.Println("----END OF INCOMING REQUEST----")
}

func sortCurrentDB(dbs []string, currentDB string) []string {
	handler.PromoteFileToFront(dbs, currentDB)
	return dbs
}

// createMainTemplate creates and parses the HTML template.
func createMainTemplate(path string) (*template.Template, error) {
	return template.New("base").Funcs(template.FuncMap{
		"TagsWithPoundList": txt.TagsWithPoundList,
		"RelativeISOTime":   txt.RelativeISOTime,
		"sortCurrentDB":     sortCurrentDB,
		"now":               func() int64 { return time.Now().UnixNano() },
		"add":               func(a, b int) int { return a + b },
		"sub":               func(a, b int) int { return a - b },
		"tagURL": func(p *RequestParams, tag string, path string) string {
			return p.With().Tag(tag).Build(path)
		},
		"letterToggleURL": func(p *RequestParams, targetLetter, path string) string {
			if p.Letter == targetLetter {
				return p.With().Letter("").Build(path)
			}
			return p.With().Letter(targetLetter).Build(path)
		},
		"seq": func(start, end int) []int {
			if start > end {
				return nil
			}
			s := make([]int, end-start+1)
			for i := range s {
				s[i] = start + i
			}
			return s
		},
	}).ParseGlob(filepath.Join(path, "*.gohtml"))
}

// buildTemplateData constructs the data structure for template rendering.
func buildTemplateData(
	r *http.Request,
	paginatedB []*bookmark.Bookmark,
	p *RequestParams,
	u string,
	tagsFn func() []string,
	pag PaginationInfo,
) TemplateData {
	return TemplateData{
		Params:         p,
		Bookmarks:      paginatedB,
		Pagination:     pag,
		TagGroups:      groupTagsByLetter(tagsFn()),
		App:            config.App,
		BaseURL:        u,
		CurrentPath:    r.URL.Path,
		LastVisitedURL: filterToggleURL(p, "last_visit", r.URL.Path),
		NewestURL:      filterToggleURL(p, "newest", r.URL.Path),
		OldestURL:      filterToggleURL(p, "oldest", r.URL.Path),
		FavoritesURL:   filterToggleURL(p, "favorites", r.URL.Path),
		MoreVisitsURL:  filterToggleURL(p, "more_visits", r.URL.Path),
		ClearTagURL:    p.With().Tag("").Build(r.URL.Path),
		ClearQueryURL:  p.With().Query("").Page(1).Build(r.URL.Path),
		DebugToggleURL: p.With().Debug(!p.Debug).Build(r.URL.Path),
	}
}

func extractTags(bs []*bookmark.Bookmark) []string {
	tagsMap := map[string]bool{}
	for _, b := range bs {
		for t := range strings.SplitSeq(b.Tags, ",") {
			if t != "" {
				if !tagsMap[t] {
					tagsMap[t] = true
				}
				continue
			}
		}
	}

	uniqueTags := make([]string, 0, len(tagsMap))
	for t := range tagsMap {
		uniqueTags = append(uniqueTags, t)
	}

	return uniqueTags
}

func sortBy(s string, bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	switch s {
	case "newest":
		sort.Slice(bs, func(i, j int) bool {
			return bs[i].CreatedAt > bs[j].CreatedAt
		})
		return bs
	case "oldest":
		sort.Slice(bs, func(i, j int) bool {
			return bs[i].CreatedAt < bs[j].CreatedAt
		})
		return bs
	case "last_visit":
		sort.Slice(bs, func(i, j int) bool {
			return bs[i].LastVisit > bs[j].LastVisit
		})
		return bs
	case "favorites":
		sort.Slice(bs, func(i, j int) bool {
			return bs[i].Favorite
		})
		return bs
	case "more_visits":
		sort.Slice(bs, func(i, j int) bool {
			return bs[i].VisitCount > bs[j].VisitCount
		})
	}

	return bs
}

func encodeErr(w http.ResponseWriter, statusCode int, err string) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err})
}
