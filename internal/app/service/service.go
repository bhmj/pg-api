package service

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bhmj/pg-api/internal/pkg/config"
	"github.com/bhmj/pg-api/internal/pkg/db"
	"github.com/bhmj/pg-api/internal/pkg/files"
	"github.com/bhmj/pg-api/internal/pkg/log"
	"github.com/bhmj/pg-api/internal/pkg/metrics"
)

// Readiness implements ready/notReady signal receiver
type Readiness interface {
	Ready()
	NotReady()
}

type service struct {
	ctx       context.Context
	cfg       *config.Config
	log       log.Logger
	readiness Readiness
	metrics   metrics.Metrics
	f         files.FileService
	// DB connection
	dbr *sql.DB
	dbw *sql.DB
	// runtime params
	version int    // API version
	method  string // HTTP method
	vpath   string // path WITH version (/v1/foo/bar/)
	path    string // path WITHOUT version (/foo/bar/)
	userID  int64  // userID if any
}

// Service implements service interface
type Service interface {
	MainHandler(w http.ResponseWriter, r *http.Request)
	FileHandler(w http.ResponseWriter, r *http.Request)
}

// NewService returns new service
func NewService(ctx context.Context, cfg *config.Config, log log.Logger, rd Readiness) Service {
	srv := &service{
		ctx:       ctx,
		cfg:       cfg,
		log:       log,
		readiness: rd,
		metrics:   metrics.NewMetrics(cfg.Service.Name, cfg.Service.Prometheus.Buckets),
	}
	// prepare database connections
	srv.dbr, _ = db.SetupDatabase(cfg.DBGroup.Read)
	dbWriteSettings, same := cfg.GetDBWrite()
	cfg.DBGroup.Write = dbWriteSettings
	if !same {
		srv.dbw, _ = db.SetupDatabase(dbWriteSettings)
	} else {
		srv.dbw = srv.dbr
	}
	srv.f = files.NewFileService(&cfg.Files, srv.dbw)

	return srv
}

// prepareParams prepares parameters for query processing
func (s *service) preprocess(w http.ResponseWriter, r *http.Request) (err error) {

	// method, paths
	s.method = r.Method
	s.vpath = r.URL.Path
	if s.vpath[len(s.vpath)-1] != '/' {
		s.vpath += "/"
	}
	// user ID if any
	if s.userID, err = s.getUserID(r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	// API version & path
	path := r.URL.Path[len(s.cfg.HTTP.Endpoint)+2:]
	subs := regexpMap["version"].FindStringSubmatch(path)
	if subs == nil {
		http.Error(w, "API version not specified", http.StatusBadRequest)
		return
	}
	s.version, _ = strconv.Atoi(subs[1])
	if s.version == 0 {
		http.Error(w, "invalid API version", http.StatusBadRequest)
		return
	}
	s.path = path[len(subs[0]):]
	if s.path == "" {
		http.Error(w, "service method not specified", http.StatusBadRequest)
		return
	}
	if s.path[len(s.path)-1] != '/' {
		s.path += "/"
	}
	// HIT ?
	r.ParseForm()
	if r.FormValue("latitude") != "" && r.FormValue("longitude") != "" {
		s.method = "HIT"
	}
	return
}

// MainHandler implements service logic
func (s *service) MainHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	t := time.Now()
	defer s.metrics.Score(s.method, s.vpath, "total", t, &err)
	// CORS
	if r.Method == "OPTIONS" && s.cfg.HTTP.CORS {
		s.allowCORS(w)
		return
	}
	// preprocess
	if err = s.preprocess(w, r); err != nil {
		return
	}
	// process
	code, err := s.processQuery(w, r)
	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}

	fmt.Fprintf(w, "r.URL.Path = %s\ns.vpath = %s, s.path = %s, s.method = %s\n", r.URL.Path, s.vpath, s.path, s.method)
}

// FileHandler implements file storage logic
func (s *service) FileHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	t := time.Now()
	defer s.metrics.Score(s.method, s.vpath, "total", t, &err)
	// CORS
	if r.Method == "OPTIONS" && s.cfg.HTTP.CORS {
		s.allowCORS(w)
		return
	}
	// preprocess
	if err = s.preprocess(w, r); err != nil {
		return
	}
	// process
	switch r.Method {
	case "POST":
		s.f.UploadFile(s.userID, w, r)
	case "GET":
		s.f.GetFile(s.userID, w, r)
	default:
		io.WriteString(w, "Only POST and GET are supported!")
		return
	}
}
