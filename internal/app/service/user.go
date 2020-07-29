package service

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// getUserID retrieves user ID or error
func (s *service) getUserID(r *http.Request) (uID int64, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
			return
		}
	}()
	return s.wrappedGetUserID(r)
}

func (s *service) wrappedGetUserID(r *http.Request) (uID int64, err error) {
	if s.cfg.Auth.CookieName == "" {
		return
	}
	cookie, err := r.Cookie(s.cfg.Auth.CookieName)
	if err != nil {
		return
	}
	if cookie == nil || cookie.Value == "" {
		err = errors.New("empty cookie, authentication failed")
		return
	}
	val := cookie.Value
	if !s.cfg.Auth.Unescaped {
		val, err = url.QueryUnescape(val)
		if err != nil {
			return
		}
	}
	items := strings.SplitN(val[s.cfg.Auth.Offset:], s.cfg.Auth.Separator, -1)
	item := items[s.cfg.Auth.Part]

	rows, err := s.dbr.Query("select * from "+s.cfg.Auth.Procedure+"($1)", item)
	if err != nil {
		return
	}
	defer rows.Close()

	var userID sql.NullInt64
	var code sql.NullInt64
	for rows.Next() {
		if err = rows.Scan(&userID, &code); err != nil {
			return
		}
	}
	if !userID.Valid || code.Int64 != 200 {
		err = errors.New("authentication failed")
	} else {
		uID = userID.Int64
	}
	return
}
