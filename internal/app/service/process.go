package service

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/bhmj/pg-api/internal/pkg/config"
	"github.com/bhmj/pg-api/internal/pkg/str"
)

// queryResult holds query result
type queryResult struct {
	Code    int    `json:"httpcode"` // synonyms
	ErrCode int    `json:"errcode"`  // synonyms
	Error   string `json:"error"`
	ID      int64  `json:"id"`
}

type Header struct {
	Name  string
	Value string
	Type  string
}

func (s *service) processQuery(w http.ResponseWriter, r *http.Request) (code int, err error) {
	code = http.StatusBadRequest
	// parse URL
	parsed, err := s.parseURL(s.path, s.version, s.cfg)
	if err != nil {
		return
	}

	body, _ := ioutil.ReadAll(r.Body)

	// headers pass-through
	var headers []Header
	if len(parsed.HeadersPass) > 0 {
		headers = extractHeaders(parsed.HeadersPass, r.Header)
		body = passImmediateHeaders(body, headers)
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

	// enhance if needed (only for standard scenario)
	if len(parsed.FinalizeName) == 0 && len(parsed.Enhance) > 0 && s.method == "POST" {
		// pre-processing
		body = s.enhanceData(body, parsed.Enhance, 1*time.Second)
	}

	db := s.dbr
	schema := s.cfg.DBGroup.Read.Schema
	if writeDB[s.method] {
		db = s.dbw
		schema = s.cfg.DBGroup.Write.Schema
	}

	// prepare main function
	query := s.prepareSQL(schema, parsed, string(body), headers, 0)

	// call main function
	var result string
	err = s.makeDBRequest(db, query, &result)
	if err != nil {
		code = http.StatusInternalServerError
		return
	}

	// error + http code from query
	var qRes queryResult
	err = json.Unmarshal([]byte(result), &qRes)
	if qRes.ErrCode > qRes.Code {
		qRes.Code = qRes.ErrCode
	}
	if qRes.Error != "" {
		s.log.L().Errorf("error: %s, query: %s", qRes.Error, query)
		code = qRes.Code
	}
	// legacy: some old fns return just ID
	if err != nil {
		i, err := strconv.ParseInt(result, 10, 64)
		if err == nil {
			qRes.ID = i
		}
	}

	rawResult := []byte(result)
	if len(parsed.FinalizeName) == 0 {
		// standard scenario: post-processing
		if len(parsed.Postproc) > 0 && s.method == "POST" {
			go func(rawRes []byte, postproc []config.Enhance) {
				_ = s.enhanceData(rawRes, postproc, 60*time.Second)
			}(rawResult, parsed.Postproc)
		}
	} else {
		// fast scenario: return id from main function and do the pre- and post-processing in the background
		go func(
			rawBody []byte,
			parsed ParsedURL,
			id int64,
		) {
			var body []byte
			var result string

			if len(parsed.Enhance) > 0 && s.method == "POST" {
				// pre-processing
				body = s.enhanceData(rawBody, parsed.Enhance, 60*time.Second)
			}

			// finalizing query
			query := s.prepareSQL(s.cfg.DBGroup.Write.Schema, parsed, string(body), headers, id)
			err = s.makeDBRequest(s.dbw, query, &result)
			if err != nil {
				s.log.L().Errorf("finalizing query: %s, error: %s", query, err.Error())
			} else {
				s.log.L().Infof("finalizing query result: %s", result)
			}
			if len(parsed.Postproc) > 0 && s.method == "POST" {
				// post-processing
				_ = s.enhanceData([]byte(result), parsed.Postproc, 60*time.Second)
			}
		}(body, parsed, qRes.ID)
	}

	// http response code
	code = qRes.Code
	if code == 0 {
		code = httpCodes[s.method]
	}

	if s.cfg.HTTP.CORS {
		s.allowCORS(w)
	}
	w.Header().Set("Content-Type", str.Scoalesce(parsed.ContentType, "application/json"))
	w.Header().Set("Content-Length", strconv.Itoa(len(rawResult)))
	w.WriteHeader(code)
	w.Write(rawResult)

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

func extractHeaders(headersToPass []config.HeaderPass, headers http.Header) []Header {
	result := make([]Header, len(headersToPass))
	for i := range headersToPass {
		canonicalHeaderKey := http.CanonicalHeaderKey(headersToPass[i].Header)
		val, found := headers[canonicalHeaderKey]
		value := ""
		if found {
			value = val[0]
		}
		result[i].Name = headersToPass[i].FieldName
		result[i].Type = headersToPass[i].ArgumentType
		result[i].Value = value
	}
	return result
}

func passImmediateHeaders(body []byte, headers []Header) []byte {
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
	for i := range headers {
		if headers[i].Type == "" { // headers with empty type are considered to be passed as json fields
			body = append(body, sep)
			body = append(body, []byte(`"`+headers[i].Name+`":"`+headers[i].Value+`"`)...)
			sep = ','
		}
	}
	body = append(body, '}')
	return body
}
