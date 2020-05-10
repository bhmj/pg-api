package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Read(t *testing.T) {
	cfg := New()
	// Read
	err := cfg.Read("*")
	assert.NotEqual(t, nil, err)
	err = cfg.Read("")
	assert.NotEqual(t, nil, err)
}

func Test_Validate(t *testing.T) {
	// invalid json + env substitution
	cfg := New()
	dummy := strings.NewReader(`{{foo}}`)
	err := cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// MaxConn
	cfg = New()
	dummy = strings.NewReader(`{
		"DBGroup":{ "Read": { "MaxConn": -1 } }
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// Service.Version
	cfg = New()
	dummy = strings.NewReader(`{
		"DBGroup":{ "Read": { "MaxConn": 0 } }
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// Service.Name 1
	cfg = New()
	dummy = strings.NewReader(`{
		"Service":{"Version":"1.0.0"}
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// Service.Name 1
	cfg = New()
	dummy = strings.NewReader(`{
		"Service":{"Version":"1.0.0", "Name":"abc def"}
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// HTTP.Endpoint
	cfg = New()
	dummy = strings.NewReader(`{
		"Service":{"Version":"1.0.0", "Name":"dummy"}
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// Methods.Name: invalid regexp
	cfg = New()
	dummy = strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080},
		"Service":{"Version":"1.0.0", "Name":"dummy"},
		"Methods":[{"Name":["(**"]}]
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// Methods.Name: invalid regexp
	cfg = New()
	dummy = strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080},
		"Service":{"Version":"1.0.0", "Name":"dummy"},
		"Methods":[{"Name":["(**"]}]
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// Methods.FinalizeName != Methods.Name
	cfg = New()
	dummy = strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080},
		"Service":{"Version":"1.0.0", "Name":"dummy"},
		"Methods":[{"Name":["aaa"],"FinalizeName":["a","b"]}]
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// HTTP.UseSSL, no Cert
	cfg = New()
	dummy = strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080, "UseSSL":true},
		"Service":{"Version":"1.0.0", "Name":"dummy"}
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// HTTP.UseSSL, invalid Cert
	cfg = New()
	dummy = strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080, "UseSSL":true, "SSLCert":"foo"},
		"Service":{"Version":"1.0.0", "Name":"dummy"}
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// HTTP.UseSSL, no Key
	cfg = New()
	dummy = strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080, "UseSSL":true, "SSLCert":"."},
		"Service":{"Version":"1.0.0", "Name":"dummy"}
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// HTTP.UseSSL, invalid Key
	cfg = New()
	dummy = strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080, "UseSSL":true, "SSLCert":".", "SSLKey":"foo"},
		"Service":{"Version":"1.0.0", "Name":"dummy"}
	}`)
	err = cfg.readIO(dummy)
	assert.NotEqual(t, nil, err)
	// complete
	cfg = New()
	dummy = strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080},
		"Service":{"Version":"1.0.0", "Name":"dummy"}
	}`)
	err = cfg.readIO(dummy)
	assert.Equal(t, nil, err)
}

func Test_GetDBWrite(t *testing.T) {
	// same DB
	cfg := New()
	_, same := cfg.GetDBWrite()
	assert.Equal(t, same, true)
}

func Test_ValidateEnhance(t *testing.T) {
	// IncomingFields <-> ForwardFields
	err := validateEnhance("foo", []Enhance{{IncomingFields: []string{"a", "b"}, ForwardFields: []string{"d"}}})
	assert.NotEqual(t, err, nil)
	// ForwardFields: [] + "x"
	err = validateEnhance("foo", []Enhance{{IncomingFields: []string{"a", "b"}, ForwardFields: []string{"[]", "a"}}})
	assert.NotEqual(t, err, nil)
	// TransferFields
	err = validateEnhance("foo", []Enhance{{TransferFields: []TransferFields{{From: "%0"}}}})
	assert.NotEqual(t, err, nil)
}

func Test_MethodProperties(t *testing.T) {
	cfg := New()
	dummy := strings.NewReader(`{
		"HTTP":{"Endpoint":"api", "Port":8080},
		"Service":{"Version":"1.0.0", "Name":"dummy"},
		"Methods":[{"Name":["foo"], "FinalizeName":["fin"], "Enhance":[{}], "Postproc":[{}], "HeadersPass":[{"Header":"foo","Value":"bar"}]}]
	}`)
	err := cfg.readIO(dummy)
	assert.Equal(t, err, nil)
	cfg.MethodProperties("foo", 1)
}
