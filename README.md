# PG-API

## What is it?

PG-API is a universal highly customizable REST API constructor for PostgreSQL.
With it, you can build a sophisticated API for a PostgreSQL database and implement business logic using stored procedures (functions).  

Main features:  
 - GET/POST/PUT/PATCH/DELETE queries
 - Key or cookie-based authorization
 - per-method versioning
 - ability to pass HTTP headers into function
 - external service calling, postponed processing
 - Prometheus metrics
 - Kubernetes ready (readiness/liveness probes, graceful shutdown)
 - MinIO file operations support
 - CORS support

## Getting started (in 5 simple steps)

#### 1. Install

```bash
$ go get github.com/bhmj/pg-api
$ cd cmd/pg-api
$ go build .
```

#### 2. Configure

create config file `dummy.json`:
```json
{
	"Service": {
		"Version": "1.0.0",
		"Name": "dummy"
	},
	"HTTP": {
		"Port": 8080,
		"Endpoint": "api"
	},
	"DBGroup": {
		"Read": {
            "ConnString": "host=localhost port=5432 dbname=postgres user=postgres password=postgres sslmode=disable",
			"Schema": "api"
		}
	}
}
```

#### 3. Write some PostgreSQL code

```SQL
create or replace function api.hello_get(int, _data json)
returns json
language plpgsql
as $$
declare
    _str text;
begin
    _str := 'Hello there, '||coalesce(_data->>'name', 'stranger')||'!';
    return json_build_object('greeting', _str);
end
$$;
```

#### 4. Run PG-API

```bash
$ ./pg-api dummy.json
```

#### 5. Your new API method is working

```bash
$ curl http://localhost:8080/api/v1/hello?name=Mike

{"greeting" : "Hello there, Mike!"}
```

## Spec

To run a PG-API you need to configure the following parts:
- Service name and version
- HTTP endpoint
- Database connection
- Methods and their properties
  - Calling convention
  - Content type (optional)
  - Headers passthrough (optional)
  - External services (optional)
  - Finalization function (optional)
- Authentication parameters (optional)
- File upload (optional)

### Config file

You may:  
a) set an environment variable `PG_API_CONFIG` with the config file path  
b) specify a config file path as the (only) command line parameter

#### Env variables substitution

You may use `{{THIS_SYNTAX}}` in config file to create a substitutions for env variables.  
Example:
```json
{ "Password": "{{SECRET}}" }
```
Thus, if the environment variable `SECRET` is set to `abc123`, the above line will be translated at runtime into
```json
{ "Password": "abc123" }
```
#### Minimal required fields

`$.Service.Name` : for metrics  
`$.Service.Version` : for distinction  
`$.HTTP.Port` : port to listen to  
`$.HTTP.Endpoint` : endpoint base  
`$.DBGroup.Read.ConnString` : DB connection. *Write* queries will use the same.  
`$.DBGroup.Read.Schema` : DB schema containing API functions

#### Default values

Convention : `CRUD`  
Content-Type : `application/json`  
CORS : `disabled`  
Authorization : `none`  
Prometheus buckets : `1ms to 5s logarithmic scale`  
Open connections : `unlimited`  
Idle connections : `none`  

### HTTP endpoint
```Go
HTTP struct {
	Endpoint    string   // API endpoint
	Port        int      // port to listen to
	UseSSL      bool     // use SSL
	SSLCert     string   // SSL certificate file path
	SSLKey      string   // SSL private key file path
	AccessFiles []string // list of files containing key + name for basic HTTP key auth
	CORS        bool     // allow CORS
}
```
### Database connection
```Go
DBGroup struct {    // Database connections
	Read  Database  // Read database params
	Write Database  // Write database params (may omit if the same)
}
```
```Go
Database struct {
    ConnString string  // instant connection string
                       // OR
    Host       string  // parts
	Port       int     // to be
	Name       string  // combined
	User       string  // at
	Password   string  // runtime :)
	Schema     string  // schema containing all the API functions
	MaxConn    int     // set this to limit the number of open connections
}
```
### Methods and their properties

```Go
MethodConfig struct {
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
```

#### Content type

Default content-type is `application/json` but it is possible to set any other, like `application/xml`, `text/html`, `text/plain` and  include charset info if needed: `application/xml; charset="UTF-8"`

#### Headers passthrough

It is possible to configure a passthrough for any number of headers (per method). A field name must be assigned for each header. Header field overwrites input (body or URL) field if names match.

```Go
HeaderPass struct {
	Header    string  // header to pass
	FieldName string  // field name in our incoming JSON
}
```

#### Calling convention

There are two possible calling conventions: `POST` and `CRUD`  
`CRUD` (default):
- GET method reads, POST, PUT, PATCH and DELETE write.
- function suffixes: `get`, `ins`, `upd`, `pat` and `del` respectively.
- intended for classic REST API (object manipulation)

`POST`:
- any HTTP method calls the same function. POST is preferred.
- as a result, no suffix on functions.
- all calls are *write* calls (use *write* database connection).
- intended for json-intensive API where any call may lead to write operations.

#### External services 

`Enhance` optional section in method definition contains external services info and a set of rules for data enrichment (only works for `POST` calling convention).

It is possible to get data from several services successively. The data received from one service will be available for sending in the next one and so on.

Section example:   
```json
"Enhance": [ // array: may contain many external service definitions
    {
        "URL"            : "http://some.service/api/",    // external service URL
        "Method"         : "POST",                        // POST or GET
        "IncomingFields" : ["$.nm_id", "$.chrt_id"],      // fields from incoming query, jsonpath
        "ForwardFields"  : ["nms", "chrts"],              // corresponding field names for external service, plain text
        "TransferFields" : [                              // data transformation rules:
            { "From": "$.result.details[0].shk_id",  "To": "shk_id" },      // From: jsonpath for received external data
            { "From": "$.result.details[0].brand",   "To": "brand_name" },  // To: field name to be added to our json
            { "From": "$.result.details[0].%2.size", "To": "size_name" }    // %2: you may use %x to use a ForwardField value in a jsonpath, by its ordinal number
        ]
    }
]
```
In case of POST method the data is passed via `json` in request body.  
In case of GET method the data is passed via URL in form of `param=value` pairs.  
A reply from the external service is expected to be a JSON.  

The result of a processing will be a JSON extended with the data received from all the sources. Any errors during external service calling or data enrichment are ignored.

#### Finalization function (optional)

#### Authentication parameters (optional)

#### File upload (optional)

#### General method definition

You can specify common parameters in `General` section. Fields which are not specified in `Methods` will be taken from `General`. All the methods which do not have matches in `Methods[:].Name` will be executed with `General` settings.

## Calling conventions

`domain:port / {endpoint} / {version} / {path} ? {params}`

| Part | Format | Description |
|---|---|---|
|**{endpoint}** | `$.HTTP.Endpoint` | usually "api" |
|**{version}** | `v[0-9]+` | a mandatory version specifier |
|**{path}** | `(/blabla/[0-9]*)+` | objects and ids |
|**{params}** | `param=value & ...` | URL params |

### Translation rules in examples

|**`CRUD`**:  |  |  |
|---|--|---|
|`GET /api/v1/foo/7/bar/9`| --> |`foo_bar_get(7,9)` |
|`GET /api/v1/foo/bar/12` | --> | `foo_bar_get(0,12)` |
|`GET /api/v1/foo/bar` | --> | `foo_bar_get(0,0)` |
|`GET /api/v1/foo/bar/3?p=v` | --> | `foo_bar_get(0,3,'{"p":"v"}')` |
|`POST /api/v1/foo/12/bar/` | --> | `foo_bar_ins(0,12,'{...}')` |
|`PUT /api/v3/foo/bar/12` | --> | `foo_bar_upd_v3(0,12,'{...}')` |
|`DELETE /api/v3/foo/bar/12` | --> | `foo_bar_del_v3(0,12)` |  

|**`POST`**:  |  |  |
|---|--|---|
|`POST /api/v1/foo/bar`| --> |`foo_bar(0,'{...}')` |
|`POST /api/v1/foo/9/bar`| --> |`foo_bar(9,'{...}')` |
|`POST /api/v3/profile?entry=FOO` | --> | `profile_v3('{"entry":"FOO", ...}')` |
|`GET /api/v1/foo/bar` | --> | `foo_bar(0,0,'{...}')` |
| NB: GET method not recommended | | |

## Changelog

**0.3.0** (2020-05-08) -- First public opensource release.

## Roadmap

- [x] method versioning
- [x] external service calling
- [x] finalizing functions
- [x] universal metrics
- [x] CORS support
- [x] headers passthrough
- [x] key- of cookie based authorization
- [x] MinIO support
- [ ] circuit breaker
- [ ] CSV / XLSX export from table functions

## Contributing

1. Fork it!
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request :)

## Licence

[MIT](http://opensource.org/licenses/MIT)

## Author

Michael Gurov aka BHMJ
