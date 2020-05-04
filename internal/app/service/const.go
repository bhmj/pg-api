package service

import "regexp"

var regexpMap = map[string]*regexp.Regexp{
	"parseUrl":         regexp.MustCompile(`(?i)(\w+)(?:/(\d+)?)`),
	"extServiceName":   regexp.MustCompile(`^.+://[^/]+/([^/?]+(?:/[^/?]+)*)/?(?:\?[^?]*)?$`),
	"splitExtServName": regexp.MustCompile(`\w+`),
	"version":          regexp.MustCompile(`v(\d+)/`),
	"validServiceName": regexp.MustCompile(`^[A-Za-z_\-]+$`),
}
