package service

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bhmj/pg-api/internal/pkg/str"
)

// prepareSQL prepares SQL
func (s *service) prepareSQL(schema string, parsed ParsedURL, body string, headers []Header, id int64) (query string) {
	suffix := suffixMap[s.method]
	var functionName string
	//id > 0 indicates that the finalizing SQL query is prepared
	if id > 0 {
		functionName = parsed.FinalizeName[0]
	} else {
		functionName = parsed.QueryPath
	}

	if parsed.Convention == "CRUD" {
		functionName += "_" + suffix
	} else {
		suffix = "ins" // use last ID in function call
	}

	// complete SQL query
	var arguments []string
	if s.userID > 0 {
		arguments = append(arguments, fmt.Sprintf("%d", s.userID))
	}
	if len(headers) > 0 {
		arguments = append(arguments, serializeHeaders(headers)...)
	}
	if len(parsed.ID) > 1 {
		arguments = append(arguments, str.CommaSeparatedString(parsed.ID[0:len(parsed.ID)-1]))
	}
	if suffix != "ins" {
		arguments = append(arguments, fmt.Sprintf("%d", parsed.ID[len(parsed.ID)-1]))
	}
	if suffix != "del" && len(body) > 0 {
		arguments = append(arguments, fmt.Sprintf("'%s'", sanitizeString(body)))
	}

	functionParams := strings.Join(arguments, ", ")
	if id > 0 {
		functionParams = strconv.FormatInt(id, 10) + ", " + functionParams // Insert id into the first position of parameters list
	}

	ver := ""
	if s.version > 1 {
		ver = "_v" + strconv.Itoa(s.version)
	}
	query = "select * from " + schema + "." + functionName + ver + " (" + functionParams + ")"

	s.log.L().Infof("SQL: %s", query)

	return
}

// makeDBRequest performs request to database
func (s *service) makeDBRequest(db *sql.DB, query string, result *string) (err error) {
	t := time.Now()
	defer s.metrics.Score(s.method, s.vpath, "db", t, &err)
	rows, err := db.Query(query)
	if err != nil {
		return
	}
	defer rows.Close()
	rows.Next()
	err = rows.Scan(result)
	return
}

func serializeHeaders(headers []Header) []string {
	result := make([]string, 0)
	numKey := map[string]bool{"int": true, "integer": true, "bigint": true, "float": true, "number": true}
	strKey := map[string]bool{"text": true, "string": true, "varchar": true}
	for i := range headers {
		if headers[i].Type == "" {
			continue
		}
		switch {
		case numKey[strings.ToLower(headers[i].Type)]:
			result = append(result, sanitizeNumber(headers[i].Value))
		case strKey[strings.ToLower(headers[i].Type)]:
			result = append(result, "'"+sanitizeString(headers[i].Value+"'"))
		}
	}
	return result
}

func sanitizeString(s string) string {
	return strings.Replace(s, "'", "''", -1)
}

func sanitizeNumber(s string) string {
	reg := regexp.MustCompile("[^0-9Ee.-]+")
	result := reg.ReplaceAllString(s, "")
	if len(result) == 0 {
		result = "0"
	}
	return result
}
