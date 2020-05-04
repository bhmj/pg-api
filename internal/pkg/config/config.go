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

	"github.com/bhmj/pg-api/internal/pkg/str"
	"github.com/bhmj/pg-api/internal/pkg/tag"
)

var validServiceName *regexp.Regexp = regexp.MustCompile(`^[A-Za-z_\-]+$`)

// HTTP defines server parameters
type HTTP struct {
	Endpoint    string   // API endpoint
	Port        int      // port to listen
	UseSSL      bool     // use SSL
	SSLCert     string   // SSL certificate file path
	SSLKey      string   // SSL private key file path
	AccessFiles []string // list of files containing key + name for basic HTTP key auth
	CORS        bool     // allow CORS
}

// Config contains all parameters
type Config struct {
	HTTP    HTTP     // HTTP params + API endpoint
	DBGroup struct { // Database connections
		Read  Database // Read database params
		Write Database // Write database params
	}
	Cache struct { //
		Enable bool
		TTL    int
	}
	Service struct {
		Name       string
		Version    string
		Prometheus struct {
			Buckets []float64
			Start   float64
			Width   float64
			Count   int
		}
		Log string
	}
	Auth struct {
		CookieName string // name of the cookie containing token
		Unescaped  bool   // in a rare case of storing unescaped cookie
		Offset     int    // defines a substring offset
		Separator  string // defines a substring separator
		Part       int    // defines a substring part number
		Procedure  string // user retrieval procedure as in "select user_id from Procedure(substring)")
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
	Name         []string     // method name
	VersionFrom  int          // method version which other params are applied from
	FinalizeName []string     // finalizing method name (omittable)
	Convention   string       // calling convention: POST, CRUD (default is CRUD)
	ContentType  string       // return content type (default is application/json)
	Enhance      []Enhance    // enhance data using external service(s)
	Postproc     []Enhance    // data postprocessing using external service(s)
	HeadersPass  []HeaderPass // pass specified headers into proc
	// runtime
	NameMatch []*regexp.Regexp // method mask(s) -- runtime
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

// HeaderPass defines Header -> FieldName mapping
type HeaderPass struct {
	Header    string
	FieldName string
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
		if !validServiceName.MatchString(t.Service.Name) {
			return fmt.Errorf("%s can contain only [a-zA-Z_-]", t.getName("Service.Name"))
		}
	} else {
		return fmt.Errorf("Service.Name is not specified")
	}

	if t.HTTP.Endpoint == "" {
		return fmt.Errorf("HTTP.Endpoint is not specified")
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
	rx, _ := regexp.Compile(`%\d+`)
	for _, enh := range enhs {
		// Method[:].Enhance.IncomingFields must match Method[:].Enhance.ForwardFields
		if len(enh.IncomingFields) != len(enh.ForwardFields) {
			return fmt.Errorf("%s: count(Enhance.IncomingFields) != count(Enhance.ForwardFields) [%d != %d]", method, len(enh.IncomingFields), len(enh.ForwardFields))
		}
		// ForwardFields array mode
		for _, fw := range enh.ForwardFields {
			if fw == "[]" && len(enh.ForwardFields) > 1 {
				return fmt.Errorf("%s: \"[]\" must be the only element in Enhance.ForwardFields", method)
			}
		}
		// TransferFields[:].From may contain references to ForwardFields[]
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

// MethodProperties returns completed MethodConfig for given version + method
func (t *Config) MethodProperties(method string, version int) MethodConfig {

	bestVer := 0         // The best version number isn't yet selected
	var bestVerIdx int   // bestVer index in t.Methods
	var finName []string // Function name to be selected from FinalizeName; this name will be the only element in the slice
	var finNameIdx int   // finName index in selected FinalizeName
	// Other params default values
	conv := t.General.Convention
	ctype := t.General.ContentType
	enhnc := t.General.Enhance
	postpr := t.General.Postproc
	hpass := t.General.HeadersPass

	// The best version number is the maximum one of all version numbers
	// in t.Methods that are not greater than version number in HTTP request.
	// t.Methods[i].VersionFrom is always > 0 (see function readConfig).
	for idx, ms := range t.Methods {
		for n, mname := range ms.NameMatch {
			if mname.MatchString(method) {
				if ms.VersionFrom <= version && ms.VersionFrom > bestVer {
					bestVer = ms.VersionFrom
					bestVerIdx = idx
					finNameIdx = n
				}
			}
		}
	}

	// If in the end the best version number was selected from t.Methods
	if bestVer > 0 {
		bestMethod := t.Methods[bestVerIdx]
		if len(bestMethod.FinalizeName) > 0 {
			finName = make([]string, 1)
			finName[0] = bestMethod.FinalizeName[finNameIdx]
		}
		if bestMethod.Convention != "" {
			conv = bestMethod.Convention
		}
		if bestMethod.ContentType != "" {
			ctype = bestMethod.ContentType
		}
		if len(bestMethod.Enhance) > 0 {
			enhnc = append(enhnc, bestMethod.Enhance...)
		}
		if len(bestMethod.Postproc) > 0 {
			postpr = append(postpr, bestMethod.Postproc...)
		}
		if len(bestMethod.HeadersPass) > 0 {
			hpass = bestMethod.HeadersPass
		}
	}

	return MethodConfig{FinalizeName: finName, Convention: conv, ContentType: ctype, Enhance: enhnc, Postproc: postpr, HeadersPass: hpass}
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

	// defaults and adjustments
	for i, p := range cfg.Methods {
		// If version number of method p is not explicitly specified
		if p.VersionFrom == 0 {
			cfg.Methods[i].VersionFrom = 1
		}
	}
	cfg.LogLevel = uint(cfg.Debug) // legacy

	if err = cfg.validate(); err != nil {
		return nil, err
	}

	//logger.Log("msg", "debug level", "debug", settings.Debug)

	return &cfg, nil
}

// GetDBWrite returns config for write db and bool indicating it is the same db as read db
func (t *Config) GetDBWrite() (Database, bool) {
	v := Database{}
	v.ConnString = str.Scoalesce(t.DBGroup.Write.ConnString, t.DBGroup.Read.ConnString)
	v.Host = str.Scoalesce(t.DBGroup.Write.Host, t.DBGroup.Read.Host)
	v.Port = str.Icoalesce(t.DBGroup.Write.Port, t.DBGroup.Read.Port)
	v.Name = str.Scoalesce(t.DBGroup.Write.Name, t.DBGroup.Read.Name)
	v.User = str.Scoalesce(t.DBGroup.Write.User, t.DBGroup.Read.User)
	v.Password = str.Scoalesce(t.DBGroup.Write.Password, t.DBGroup.Read.Password)
	v.Schema = str.Scoalesce(t.DBGroup.Write.Schema, t.DBGroup.Read.Schema)
	v.MaxConn = str.Icoalesce(t.DBGroup.Write.MaxConn, t.DBGroup.Read.MaxConn)
	return v, (v == t.DBGroup.Read)
}
