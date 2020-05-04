package service

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/bhmj/pg-api/internal/pkg/config"
)

func (s *service) processQuery(r *http.Request) (code int, err error) {
	code = http.StatusBadRequest
	// parse URL
	parsed, err := s.parseURL(s.path, s.version, s.cfg)
	if err != nil {
		return
	}

	body, _ := ioutil.ReadAll(r.Body)

	// headers pass-through
	if len(parsed.HeadersPass) > 0 {
		body = passHeaders(body, parsed.HeadersPass, r.Header)
	}

	// enrich body JSON with URL params
	var params = make(map[string]interface{})
	for k, v := range r.URL.Query() {
		params[k] = v[0]
	}
	if len(params) > 0 {
		var bodyObj map[string]interface{}
		err = json.Unmarshal(body, &bodyObj)
		if err != nil {
			body, _ = json.Marshal(params)
		} else {
			for k, v := range params {
				bodyObj[k] = v
			}
			body, _ = json.Marshal(bodyObj)
		}
	}

	if len(parsed.FinalizeName) == 0 && len(parsed.Enhance) > 0 && s.method == "POST" {
		body = s.enhanceData(body, parsed.Enhance, 1*time.Second)
	}

	db := s.dbr
	schema := s.cfg.DBGroup.Read.Schema
	if writeDB[s.method] {
		db = s.dbw
		schema = s.cfg.DBGroup.Write.Schema
	}

	query := s.prepareSQL(schema, parsed, string(body), 0)

	var result string
	err = s.makeDBRequest(db, query, &result)
	if err != nil {
		code = http.StatusInternalServerError
		return
	}

	code = http.StatusOK
	err = nil
	return
}

func (s *service) parseURL(urlpath string, version int, cfg *config.Config) (parsed ParsedURL, err error) {
	parsed = ParsedURL{}
	var rx = regexpMap["parseUrl"]
	if !rx.MatchString(urlpath) {
		return parsed, errors.New("invalid url")
	}
	submatches := rx.FindAllStringSubmatch(urlpath, -1)
	parsed.ID = make([]int64, len(submatches))
	parsed.MethodPath = "/"
	parsed.QueryPath = ""
	for i, step := range submatches {
		parsed.MethodPath += step[1] + "/"
		parsed.QueryPath += step[1]
		if i < len(submatches)-1 {
			parsed.QueryPath += "_"
		}
		id, _ := strconv.ParseInt(step[2], 10, 64)
		parsed.ID[i] = id
	}
	props := cfg.MethodProperties(parsed.MethodPath, version)
	parsed.MethodConfig = props

	id := parsed.ID[len(parsed.ID)-1]

	if id != 0 && s.method == "POST" {
		err = errors.New("unnecessary item ID in POST query")
	}
	if id == 0 && (s.method == "PUT" || s.method == "PATCH" || s.method == "DELETE") {
		err = errors.New("item ID required")
	}

	return parsed, err
}

func passHeaders(body []byte, headersToPass []config.HeaderPass, headers http.Header) []byte {
	if len(body) == 0 {
		body = []byte{'{', '}'}
	}
	closing := len(body) - 1
	for ; closing >= 0; closing-- {
		if body[closing] == '}' {
			break
		}
		if body[closing] == ']' {
			return body
		}
	}
	if closing <= 0 {
		return body
	}
	sep := byte(',')
	for i := closing - 1; i >= 0; i-- {
		if body[i] == ' ' || body[i] == '\t' || body[i] == '\n' {
			continue
		}
		if body[i] == '{' {
			sep = ' '
			break
		}
		if i == 0 {
			return body
		}
		break
	}

	body = body[:closing]
	for i := range headersToPass {
		s := http.CanonicalHeaderKey(headersToPass[i].Header)
		if val, found := headers[s]; found {
			body = append(body, sep)
			body = append(body, []byte(`"`+headersToPass[i].FieldName+`":"`+val[0]+`"`)...)
			sep = ','
		}
	}
	body = append(body, '}')
	return body
}
