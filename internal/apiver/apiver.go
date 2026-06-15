package apiver

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrVersionNotFound      = errors.New("apiver: version not found")
	ErrHandlerNotFound      = errors.New("apiver: handler not found for version")
	ErrNoVersionExtractor   = errors.New("apiver: no version extractor configured")
	ErrInvalidVersionFormat = errors.New("apiver: invalid version format")
	ErrConverterNotFound    = errors.New("apiver: converter not found for version pair")
)

type Version string

func (v Version) String() string {
	return string(v)
}

func (v Version) Compare(other Version) int {
	numV := parseVersionNumber(v)
	numOther := parseVersionNumber(other)
	if numV < numOther {
		return -1
	} else if numV > numOther {
		return 1
	}
	return 0
}

func parseVersionNumber(v Version) int {
	s := strings.TrimPrefix(string(v), "v")
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

var validVersionPattern = regexp.MustCompile(`^v\d+$`)

func IsValidVersion(v Version) bool {
	return validVersionPattern.MatchString(string(v))
}

type VersionStrategy int

const (
	PathStrategy VersionStrategy = iota
	HeaderStrategy
	QueryStrategy
)

type VersionExtractor interface {
	ExtractVersion(r *http.Request) (Version, bool)
	Strategy() VersionStrategy
}

type PathVersionExtractor struct {
	pattern *regexp.Regexp
}

func NewPathVersionExtractor() *PathVersionExtractor {
	return &PathVersionExtractor{
		pattern: regexp.MustCompile(`^/(v\d+)(/.*)?$`),
	}
}

func (e *PathVersionExtractor) ExtractVersion(r *http.Request) (Version, bool) {
	matches := e.pattern.FindStringSubmatch(r.URL.Path)
	if len(matches) < 2 {
		return "", false
	}
	return Version(matches[1]), true
}

func (e *PathVersionExtractor) Strategy() VersionStrategy {
	return PathStrategy
}

type HeaderVersionExtractor struct {
	HeaderName string
}

func NewHeaderVersionExtractor() *HeaderVersionExtractor {
	return &HeaderVersionExtractor{
		HeaderName: "API-Version",
	}
}

func NewHeaderVersionExtractorWithName(headerName string) *HeaderVersionExtractor {
	return &HeaderVersionExtractor{
		HeaderName: headerName,
	}
}

func (e *HeaderVersionExtractor) ExtractVersion(r *http.Request) (Version, bool) {
	v := r.Header.Get(e.HeaderName)
	if v == "" {
		return "", false
	}
	return Version(v), true
}

func (e *HeaderVersionExtractor) Strategy() VersionStrategy {
	return HeaderStrategy
}

type QueryVersionExtractor struct {
	ParamName string
}

func NewQueryVersionExtractor() *QueryVersionExtractor {
	return &QueryVersionExtractor{
		ParamName: "version",
	}
}

func NewQueryVersionExtractorWithName(paramName string) *QueryVersionExtractor {
	return &QueryVersionExtractor{
		ParamName: paramName,
	}
}

func (e *QueryVersionExtractor) ExtractVersion(r *http.Request) (Version, bool) {
	v := r.URL.Query().Get(e.ParamName)
	if v == "" {
		return "", false
	}
	return Version(v), true
}

func (e *QueryVersionExtractor) Strategy() VersionStrategy {
	return QueryStrategy
}

type RequestConverter func(r *http.Request) (*http.Request, error)

type ResponseConverter func(statusCode int, header http.Header, body []byte) (int, http.Header, []byte, error)

type converterPair struct {
	From Version
	To   Version
}

type VersionRouter struct {
	mu              sync.RWMutex
	handlers        map[Version]http.HandlerFunc
	requestConvs    map[converterPair]RequestConverter
	responseConvs   map[converterPair]ResponseConverter
	extractors      []VersionExtractor
	defaultVersion  Version
	strippedPathKey contextKey
}

type contextKey string

const StrippedPathKey contextKey = "stripped_path"

func NewVersionRouter() *VersionRouter {
	return &VersionRouter{
		handlers:      make(map[Version]http.HandlerFunc),
		requestConvs:  make(map[converterPair]RequestConverter),
		responseConvs: make(map[converterPair]ResponseConverter),
		extractors: []VersionExtractor{
			NewPathVersionExtractor(),
			NewHeaderVersionExtractor(),
			NewQueryVersionExtractor(),
		},
		strippedPathKey: StrippedPathKey,
	}
}

func (vr *VersionRouter) SetExtractors(extractors ...VersionExtractor) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.extractors = make([]VersionExtractor, len(extractors))
	copy(vr.extractors, extractors)
}

func (vr *VersionRouter) GetExtractors() []VersionExtractor {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	extractors := make([]VersionExtractor, len(vr.extractors))
	copy(extractors, vr.extractors)
	return extractors
}

func (vr *VersionRouter) SetDefaultVersion(v Version) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.defaultVersion = v
}

func (vr *VersionRouter) GetDefaultVersion() Version {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	return vr.defaultVersion
}

func (vr *VersionRouter) RegisterHandler(v Version, h http.HandlerFunc) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.handlers[v] = h
}

func (vr *VersionRouter) GetHandler(v Version) (http.HandlerFunc, bool) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	h, ok := vr.handlers[v]
	return h, ok
}

func (vr *VersionRouter) RegisterRequestConverter(from, to Version, conv RequestConverter) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.requestConvs[converterPair{From: from, To: to}] = conv
}

func (vr *VersionRouter) GetRequestConverter(from, to Version) (RequestConverter, bool) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	conv, ok := vr.requestConvs[converterPair{From: from, To: to}]
	return conv, ok
}

func (vr *VersionRouter) RegisterResponseConverter(from, to Version, conv ResponseConverter) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.responseConvs[converterPair{From: from, To: to}] = conv
}

func (vr *VersionRouter) GetResponseConverter(from, to Version) (ResponseConverter, bool) {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	conv, ok := vr.responseConvs[converterPair{From: from, To: to}]
	return conv, ok
}

func (vr *VersionRouter) Versions() []Version {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	versions := make([]Version, 0, len(vr.handlers))
	for v := range vr.handlers {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Compare(versions[j]) < 0
	})
	return versions
}

func (vr *VersionRouter) LatestVersion() (Version, bool) {
	versions := vr.Versions()
	if len(versions) == 0 {
		return "", false
	}
	return versions[len(versions)-1], true
}

func (vr *VersionRouter) ExtractVersion(r *http.Request) (Version, *http.Request, error) {
	vr.mu.RLock()
	extractors := vr.extractors
	defaultVersion := vr.defaultVersion
	vr.mu.RUnlock()

	if len(extractors) == 0 {
		return "", r, ErrNoVersionExtractor
	}

	for _, extractor := range extractors {
		if v, ok := extractor.ExtractVersion(r); ok {
			if !IsValidVersion(v) {
				return "", r, ErrInvalidVersionFormat
			}
			if extractor.Strategy() == PathStrategy {
				stripped := stripVersionPrefix(r.URL.Path, v)
				newReq := r.Clone(r.Context())
				newReq.URL.Path = stripped
				ctx := context.WithValue(newReq.Context(), vr.strippedPathKey, stripped)
				newReq = newReq.WithContext(ctx)
				return v, newReq, nil
			}
			return v, r, nil
		}
	}

	if defaultVersion != "" {
		if !IsValidVersion(defaultVersion) {
			return "", r, ErrInvalidVersionFormat
		}
		return defaultVersion, r, nil
	}

	return "", r, ErrVersionNotFound
}

func stripVersionPrefix(path string, v Version) string {
	prefix := "/" + string(v)
	if path == prefix {
		return "/"
	}
	if strings.HasPrefix(path, prefix+"/") {
		return strings.TrimPrefix(path, prefix)
	}
	return path
}

type responseCapture struct {
	header      http.Header
	statusCode  int
	body        []byte
	wroteHeader bool
}

func newResponseCapture() *responseCapture {
	return &responseCapture{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (rc *responseCapture) Header() http.Header {
	return rc.header
}

func (rc *responseCapture) WriteHeader(code int) {
	if rc.wroteHeader {
		return
	}
	rc.statusCode = code
	rc.wroteHeader = true
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.body = append(rc.body, b...)
	return len(b), nil
}

func (vr *VersionRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestedVersion, req, err := vr.ExtractVersion(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	vr.mu.RLock()
	_, hasHandler := vr.handlers[requestedVersion]
	vr.mu.RUnlock()

	if !hasHandler {
		http.Error(w, ErrHandlerNotFound.Error(), http.StatusNotFound)
		return
	}

	latestVersion, hasLatest := vr.LatestVersion()
	if !hasLatest {
		http.Error(w, ErrHandlerNotFound.Error(), http.StatusNotFound)
		return
	}

	if requestedVersion.Compare(latestVersion) == 0 {
		vr.mu.RLock()
		handler := vr.handlers[requestedVersion]
		vr.mu.RUnlock()
		handler(w, req)
		return
	}

	reqConv, hasReqConv := vr.GetRequestConverter(requestedVersion, latestVersion)
	if !hasReqConv {
		vr.mu.RLock()
		handler := vr.handlers[requestedVersion]
		vr.mu.RUnlock()
		handler(w, req)
		return
	}

	convertedReq, err := reqConv(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	capture := newResponseCapture()
	vr.mu.RLock()
	latestHandler := vr.handlers[latestVersion]
	vr.mu.RUnlock()
	latestHandler(capture, convertedReq)

	respConv, hasRespConv := vr.GetResponseConverter(latestVersion, requestedVersion)
	if !hasRespConv {
		http.Error(w, ErrConverterNotFound.Error(), http.StatusInternalServerError)
		return
	}

	statusCode, header, body, err := respConv(
		capture.statusCode,
		capture.header,
		capture.body,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, values := range header {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(statusCode)
	w.Write(body)
}

func StrippedPathFromContext(ctx context.Context) (string, bool) {
	path, ok := ctx.Value(StrippedPathKey).(string)
	return path, ok
}
