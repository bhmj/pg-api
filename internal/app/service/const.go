package service

import (
	"regexp"
)

// used regular expressions
var regexpMap = map[string]*regexp.Regexp{
	"parseUrl":            regexp.MustCompile(`(?i)(\w+)(?:/(\d+)?)`),                            // (word/)
	"extServiceName":      regexp.MustCompile(`^.+://[^/]+/([^/?]+(?:/[^/?]+)*)/?(?:\?[^?]*)?$`), // something://domain.com[/path/path]/[?some=params]
	"splitExtServiceName": regexp.MustCompile(`\w+`),
	"version":             regexp.MustCompile(`v(\d+)/`),
}

// procedure suffixes per method
var suffixMap = map[string]string{
	"HIT":    "hit",
	"GET":    "get",
	"POST":   "ins",
	"PUT":    "upd",
	"PATCH":  "pat",
	"DELETE": "del",
}

// default HTTP status code per method
var httpCodes = map[string]int{
	"HIT":    200,
	"GET":    200,
	"POST":   201,
	"PUT":    204,
	"PATCH":  204,
	"DELETE": 204,
}

// read/write database mapping
var writeDB = map[string]bool{
	"HIT":    false,
	"GET":    false,
	"POST":   true,
	"PUT":    true,
	"PATCH":  true,
	"DELETE": true,
}
