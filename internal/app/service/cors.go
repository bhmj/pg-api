package service

import "net/http"

func (s *service) allowCORS(w http.ResponseWriter) {
	xAuth := ""
	if len(s.cfg.HTTP.AccessFiles) > 0 {
		xAuth += ", X-Auth-Sign, X-Auth-ID"
	}
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, GET, POST, PUT, PATCH, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization"+xAuth)
}
