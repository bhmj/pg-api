package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/bhmj/pg-api/internal/pkg/tag"
)

// RegexpMap represents special mappings
var RegexpMap = map[string]*regexp.Regexp{
	"parseUrl":         regexp.MustCompile(`(?i)(\w+)(?:/(\d+)?)`),
	"extServiceName":   regexp.MustCompile(`^.+://[^/]+/([^/?]+(?:/[^/?]+)*)/?(?:\?[^?]*)?$`),
	"splitExtServName": regexp.MustCompile(`\w+`),
	"version":          regexp.MustCompile(`v(\d+)/`),
	"validServiceName": regexp.MustCompile(`^[A-Za-z_\-]+$`),
}

// NullableBool is what the name says
type NullableBool bool

// Config contains all parameters
type Config struct {
	HTTP struct {
		Endpoint    string
		Port        int
		UseSSL      bool
		SSLCert     string
		SSLKey      string
		AccessFiles []string
		CORS        bool
	}
	DBGroup struct {
		Write Database
		Read  Database
	}
	Cache struct {
		Enable bool
		TTL    int
	}
	Service struct {
		Name       string
		Version    string
		Prometheus struct {
			Start float64
			Width float64
			Count int
		}
		Log string
	}
	General MethodConfig
	Methods []MethodConfig `json:",omitempty"`
	//Pusher  pusher.Cfg
	Files struct {
		SizeLimit int64
		Host      string
		Key       string
		Pass      string
		UseSSL    bool
		Procedure string
	}
	Debug    int
	LogLevel uint // 0,1,2,3 = none, errors, warnings, verbose
	// TODO: add pid settings
}

// Database defines DB params
type Database struct {
	ConnString string
	Host       string
	Port       int
	Name       string
	User       string
	Password   string
	Schema     string
	MaxConn    int
}

// MethodConfig defines methods
type MethodConfig struct {
	Name         []string   // method name
	VersionFrom  int        // method version which other params are applied from
	FinalizeName []string   // finalizing method name (omittable)
	Convention   string     // calling convention: POST, CRUD (default is CRUD)
	ContentType  string     // return content type (default is application/json)
	Enhance      []Enhance  // enhance data using external service(s)
	Postproc     []Enhance  // data postprocessing using external service(s)
	HeadersPass  []struct { // pass specified headers into proc
		Header    string
		FieldName string
	}
	// runtime
	NameMatch []*regexp.Regexp // method mask(s) -- runtime
	// DELETE:
	// Strict       *NullableBool // use strict version control istead of dispatched call on DB side (soft backward compatibility)
}

// Enhance methods
type Enhance struct {
	URL            string     // service URL
	Method         string     // GET/POST
	Condition      string     // Condition for invoking third-party service
	IncomingFields []string   // fields from incoming query, jsonpath (ex: "$.nm_id")
	ForwardFields  []string   // fields in forwarded query, plain text (ex: "ids")
	TransferFields []struct { // response transfer: from received to target
		From string // jsonpath, based on root
		To   string // jsonpath, based on current node
	}
}

func (t *Config) getName(fieldName string) string {
	field, ok := reflect.TypeOf(t).Elem().FieldByName(fieldName)
	if ok {
		jsonName := string(tag.Parse(field.Tag.Get("json")))
		if jsonName != "" {
			return jsonName
		}
	}
	return fieldName
}

func (t *Config) validate() error {

	if t.DBGroup.Read.MaxConn < 0 {
		return fmt.Errorf("%s should be >= 0", t.getName("MaxConn"))
	}

	if t.Service.Version == "" {
		return fmt.Errorf("Service.Version is not specified")
	}

	if t.Service.Name != "" {
		if !RegexpMap["validServiceName"].MatchString(t.Service.Name) {
			return fmt.Errorf("%s can contain only [a-zA-Z_-]", t.getName("Service.Name"))
		}
	} else {
		return fmt.Errorf("Service.Name is not defined, ")
	}

	// TODO: Remove this check when cache logic will be implemented
	if t.Cache.Enable {
		//logger.Log("msg", "cache doesn't work right now")
	}

	if err := validateEnhance("General", t.General.Enhance); err != nil {
		return err
	}

	for i, item := range t.Methods {

		if err := validateEnhance(strings.Join(item.Name, ","), item.Enhance); err != nil {
			return err
		}

		t.Methods[i].NameMatch = make([]*regexp.Regexp, len(item.Name))
		for n, nm := range item.Name {
			var r *regexp.Regexp
			var err error
			if r, err = regexp.Compile(nm); err != nil {
				return fmt.Errorf("invalid regex \"%s\"", nm)
			}
			t.Methods[i].NameMatch[n] = r
		}

		if len(item.FinalizeName) > 0 && len(item.FinalizeName) != len(item.Name) {
			return fmt.Errorf("slices Name and FinalizeName in Methods[%d] have different lengths", i)
		}

	}

	if t.HTTP.UseSSL {
		if t.HTTP.SSLCert == "" {
			return fmt.Errorf("provide %s to use ssl", t.getName("SSLCert"))
		} else if _, err := os.Stat(t.HTTP.SSLCert); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", t.HTTP.SSLCert)
		}
		if t.HTTP.SSLKey == "" {
			return fmt.Errorf("provide %s to use ssl", t.getName("SSLKey"))
		} else if _, err := os.Stat(t.HTTP.SSLKey); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", t.HTTP.SSLKey)
		}
	}

	return nil
}

func validateEnhance(method string, enhs []Enhance) error {
	for _, enh := range enhs {
		if len(enh.IncomingFields) != len(enh.ForwardFields) {
			return fmt.Errorf("%s: count(Enhance.IncomingFields) != count(Enhance.ForwardFields) [%d != %d]", method, len(enh.IncomingFields), len(enh.ForwardFields))
		}
		for _, fw := range enh.ForwardFields {
			if fw == "[]" && len(enh.ForwardFields) > 1 {
				return fmt.Errorf("%s: \"[]\" must be the only element in Enhance.ForwardFields", method)
			}
		}
		rx, _ := regexp.Compile(`%\d+`)
		for _, tr := range enh.TransferFields {
			for _, match := range rx.FindAllString(tr.From, -1) {
				idx, _ := strconv.Atoi(strings.Replace(match, "%", "", -1))
				if idx <= 0 || idx > len(enh.ForwardFields) {
					return fmt.Errorf("%s: unmatched wildcard \"%s\" in \"%s\"", method, match, tr.From)
				}
			}

		}
	}
	return nil
}

// Read reads config
func Read(fname string) (*Config, error) {
	// pass secrets through env
	conf, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	rx := regexp.MustCompile(`{{(\w+)}}`)
	for {
		matches := rx.FindSubmatch(conf)
		if matches == nil {
			break
		}
		v := os.Getenv(strings.ToUpper(string(matches[1])))
		conf = bytes.ReplaceAll(conf, matches[0], []byte(v))
	}

	var cfg Config
	if err := json.Unmarshal(conf, &cfg); err != nil {
		return nil, err
	}

	for i, p := range cfg.Methods {
		// If version number of method p is not explicitly specified
		if p.VersionFrom == 0 {
			cfg.Methods[i].VersionFrom = 1
		}
	}

	if err = cfg.validate(); err != nil {
		return nil, err
	}

	//logger.Log("msg", "debug level", "debug", settings.Debug)

	return &cfg, nil
}
