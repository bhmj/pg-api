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
--------------------------------------------------------------------
## Configuration

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

To specify a config file you can:   
a) set an environment variable `PG_API_CONFIG` with the config file path  
b) specify a config file path as the (only) command line parameter

## PG-API endpoints

| Endpoint | Description |
| --- | --- |
| `/metrics` | Prometheus metrics |
| `/ready` | Readiness handler for k8s. HTTP 200 for ok, 500 if not ready |
| `/alive` | Liveness handler for k8s. HTTP 200 for ok, 500 if terminating |
| `/{endpoint}/files/*` | File storage endpoint (see File operations below) |
| `/{endpoint}/v1/*` | Main endpoint (see Calling conventions below) |

## Query processing

The order of query processing in PG-API is as follows:  

If **NO** Finalizing function is specified

This is relatively simple linear scenario: [preprocessing] -> function -> return -> [postprocessing]

<img src="./docs/Case 1.svg">

If Finalizing function **IS** specified

This scenario is for quick object creation: init -> return -> [preprocessing] -> finalization -> [postprocessing]. It is useful when the preprocessing or object creation can take a considerable amount of time and the result of query (usually the object ID) is needed immediately. For example, when receiving a user review, you need to make a lot of additional processing like translation, user score, text and photo filtering and so on. This process is executed in the background but the review ID should be returned immediately. Preprocessing and postprocessing stages use ID created at init state.

<img src="./docs/Case 2.svg">

## Query parts

`{method} domain:port / {endpoint} / {version} / {path} ? {params}`  
Example: `GET` `192.168.1.1:8080` / `api` / `v1` / `hello` ? `name=Mike`

| Part | Format / Source | Description |
|---|---|---|
|**{method}** | `GET`, `POST`, `PUT`, `PATCH`, `DELETE` | available HTTP methods |
|**{endpoint}** | `$.HTTP.Endpoint` | Arbitrary word. Usually "api" |
|**{version}** | `v[0-9]+` | A mandatory version specifier |
|**{path}** | `(/blabla/[0-9]*)+` | objects and their IDs |
|**{params}** | `param=value & ...` | URL params |

### Translation rules

* **{endpoint}** is the base of all API URLs.  

* **{path}** is split into array of **object** and (optional) **ID** pairs separated by a forward slash. **object**s are then merged into a string using underscore (`_`) to make a function name. **ID**s are passed as parameters into the function. Omitted **ID**s are treated as zeros.  

* For CRUD: **{method}** translated into function suffix:
  | method | suffix|
  |---|---|
  |`GET`|`_get`|
  |`POST`|`_ins`|
  |`PUT`|`_upd`|
  |`PATCH`|`_pat`|
  |`DELETE`|`_del`|
* For POST: **{method}** is not used  

* **{version}** is applied after suffix as `_vN` only if **version is greater than 
1**.

* **{params}** are converted into "key-value" pairs and passed in the last argument as a JSON object.

* **{body}** (where applicable) must be a JSON object or array. If the body is an object, any params passed via URL are attached to the JSON (replacing same fields from body). If the body is an array, the parameters passed via URL are ignored. The resulting JSON is then passed into the DB function as a last argument.

### Translation rules in examples

|**`CRUD`**  |  |  |
|:--|--|---|
|`GET /api/v1/foo/7/bar/9`| --> |`foo_bar_get(7,9,'{}')` |
|`GET /api/v1/foo/bar/12` | --> | `foo_bar_get(0,12,'{}')` |
|`GET /api/v1/foo/bar` | --> | `foo_bar_get(0,0,'{}')` |
|`GET /api/v1/foo/bar/3?p=v` | --> | `foo_bar_get(0,3,'{"p":"v"}')` |
|`POST /api/v1/foo/12/bar/` + `{...}` as body | --> | `foo_bar_ins(12,'{...}')` |
|`PUT /api/v3/foo/12/bar/34` + `{...}` as body | --> | `foo_bar_upd_v3(12,34,'{...}')` |
|`DELETE /api/v3/foo/bar/12` | --> | `foo_bar_del_v3(0,12)` |  
|  **`POST`**  |  |  |
|`POST /api/v1/foo/bar` + `{...}` as body| --> |`foo_bar(0,'{...}')` |
|`POST /api/v1/foo/9/bar` + `{...}` as body| --> |`foo_bar(9,'{...}')` |
|`POST /api/v3/profile?entry=FOO` + `{...}` as body | --> | `profile_v3('{"entry":"FOO", ...}')` |
|`GET /api/v1/foo/bar` | --> | `foo_bar(0,0,'{}')` |
| NB: GET method not recommended | | |

--------------------------------------------------------------------

## Config file details

Config file is in JSON format. 

### Env variables substitution

You may use `{{THIS_SYNTAX}}` in config file to create a substitutions for environment variables.  
Example:
```json
{ "Password": "{{SECRET}}" }
```
Thus, if the environment variable `SECRET` is set to `abc123`, the above line will be translated at runtime into
```json
{ "Password": "abc123" }
```
### Minimal required fields

`$.Service.Name` for metrics  
`$.Service.Version` for distinction  
`$.HTTP.Port` port to listen to  
`$.HTTP.Endpoint` endpoint base  
`$.DBGroup.Read.ConnString` DB connection. *Write* queries will use the same.  
`$.DBGroup.Read.Schema` DB schema containing API functions  

see examples/minimal.json

### Default values

Convention : `CRUD`  
Content-Type : `application/json`  
CORS : `disabled`  
Authorization : `none`  
Prometheus buckets : `1ms to 5s logarithmic scale`  
Open connections : `unlimited`  
Idle connections : `none`  
LogLevel : `0` (none)  

### HTTP section
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
### Database section
```Go
DBGroup struct {
    Read  Database  // Read database params
    Write Database  // Write database params (may omit if the same)
}
```
```Go
Database struct {
    ConnString string  // instant connection string
    // --OR--
    Host       string  // parts
    Port       int     // to be
    Name       string  // combined
    User       string  // at
    Password   string  // runtime :)
    //
    Schema     string  // (mandatory) schema containing all the API functions
    MaxConn    int     // (optional) set this to limit the number of open connections
}
```
### Methods section (and their properties)

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
}
```

#### Content type

Default content-type is `application/json` but it is possible to set any other, like `application/xml`, `text/html`, `text/plain` and also to include character set info if needed: `application/xml; charset="UTF-8"`

#### HTTP Headers passthrough

It is possible to configure a passthrough for any number of header values (per method or globally). `Header` specifies a name of the header. `ArgumentType` converts a value into function argument (numeric or text). Argument headers are passed first, before object IDs and data. Empty `ArgumentType` means that the value will be passed into function as a JSON field (in the last argument). In this case a `FieldName` must be assigned. Header field overwrites input (body or URL) field of the same name.

```Go
HeaderPass struct {
    Header       string  // header to pass
    FieldName    string  // field name in our incoming JSON
    ArgumentType string  // empty or "int" or "float" or "string"
}
```
#### Calling convention types

There are two possible calling conventions: `POST` and `CRUD`  

`CRUD` (default):
- GET method read, POST, PUT, PATCH and DELETE write.
- function suffixes: `get`, `ins`, `upd`, `pat` and `del` respectively.
- intended for classic REST API (object manipulation)

`POST`:
- any HTTP method call the same function. POST is preferred.
- as a result, no suffix on functions.
- all calls are *write* calls (i.e. use *write* database connection).
- intended for json-intensive API where any call can lead to write operations.

### External services 

`Enhance` optional section in method definition contains external services info and a set of rules for data enrichment (only applicable for `POST` calling convention).

It is possible to get data from several services successively. The data received from one service will be available for sending in the next one and so on.

Section example:   
```Go
"Enhance": [ // array: may contain many external service definitions
    {
        "URL"            : "http://some.service/api/", // external service URL
        "Method"         : "POST",                     // POST or GET
        "IncomingFields" : ["$.nm_id", "$.chrt_id"],   // fields from incoming query (jsonpath)
        "ForwardFields"  : ["nms", "chrts"],           // corresponding field names *for* external service
        "TransferFields" : [                           // data transformation rules:
            { "From": "$.result.details[0].shk_id",  "To": "shk_id" },
            { "From": "$.result.details[0].brand",   "To": "brand_name" },
            { "From": "$.result.details[0].%2.size", "To": "size_name" }
            // From: jsonpath for received external data
            // To: field name to be added to our json
            // %2: you may use %x to use a ForwardField value in a jsonpath, by its ordinal number
        ]
    }
]
```
In case of POST method the data is passed via `json` in request body.  
In case of GET method the data is passed via URL in form of `param=value` pairs.  
A reply from the external service is expected to be a JSON.  

The result of a processing will be a JSON extended with the data received from all the sources. Any errors during external service calling or data enrichment are ignored.

#### Preprocessing / postprocessing

#### Finalization function (optional)

#### Authentication parameters (optional)

#### File upload (optional)

#### General method definition

You can specify common parameters in `General` section. Fields which are not specified in `Methods` will be taken from `General`. All the methods which do not have matches in `Methods[:].Name` will be executed with `General` settings.

---------------------------------------------------------------------

## More examples

See `examples/` directory for some real-life configuration files taken from production environment.

Disclaimer: All meaningful values in above examples have been replaced. All passwords, user names, server names and field names in above examples are entirely fictional.

---------------------------------------------------------------------

## Changelog

**0.4.1** (2021-02-13) -- HTTP headers can be passed to multipart/form data (for minio)

**0.4.0** (2021-02-08) -- HTTP headers can be passed as function arguments

**0.3.0** (2020-05-07) -- First public opensource release.

## Roadmap

- [x] method versioning
- [x] external service calling
- [x] finalizing functions
- [x] universal metrics
- [x] CORS support
- [x] headers passthrough
- [x] key- or cookie-based authorization
- [x] MinIO support
- [x] Enhance[:].InArray
- [x] Enhance[:].HeadersToSend
- [x] YAML config
- [ ] tests!
- [ ] more examples, explained
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
