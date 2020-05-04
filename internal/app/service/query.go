package service

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bhmj/pg-api/internal/pkg/str"
)

// prepareSQL prepares SQL
func (s *service) prepareSQL(schema string, parsed ParsedURL, body string, id int64) (query string) {
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
	var parameters []string
	if s.userID > 0 {
		parameters = append(parameters, fmt.Sprintf("%d", s.userID))
	}
	if len(parsed.ID) > 1 {
		parameters = append(parameters, str.CommaSeparatedString(parsed.ID[0:len(parsed.ID)-1]))
	}
	if suffix != "ins" {
		parameters = append(parameters, fmt.Sprintf("%d", parsed.ID[len(parsed.ID)-1]))
	}
	if suffix != "del" && len(body) > 0 {
		parameters = append(parameters, fmt.Sprintf("'%s'", strings.Replace(body, "'", "''", -1)))
	}

	functionParams := strings.Join(parameters, ", ")
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
	defer s.metrics.Score(s.method, s.path, "db", t, &err)
	rows, err := db.Query(query)
	if err != nil {
		return
	}
	defer rows.Close()
	rows.Next()
	err = rows.Scan(result)
	return
}
