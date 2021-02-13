package files

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bhmj/pg-api/internal/pkg/config"
	phttp "github.com/bhmj/pg-api/internal/pkg/http"
	"github.com/bhmj/pg-api/internal/pkg/log"
	"github.com/minio/minio-go"
)

type tUserKey int

const (
	userKey tUserKey = 0
	// MB is Megabyte
	MB = 1 << 20
)

type fileService struct {
	mcli        *minio.Client
	cfg         *config.Minio
	dbw         *sql.DB
	log         log.Logger
	base        string
	headersPass []config.HeaderPass
}

// FileService implements file service API
type FileService interface {
	UploadFile(w http.ResponseWriter, r *http.Request)
	GetFile(w http.ResponseWriter, r *http.Request)
}

// NewFileService returns new file service interface
func NewFileService(cfg *config.Minio, db *sql.DB, log log.Logger, base string, headersPass []config.HeaderPass) (FileService, error) {
	// Initialize minio client
	minioClient, err := minio.New(cfg.Host, cfg.AccessKey, cfg.SecretKey, cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	return &fileService{
		log:         log,
		cfg:         cfg,
		dbw:         db,
		mcli:        minioClient,
		base:        base,
		headersPass: headersPass,
	}, nil
}

// GetFile returns a file
func (s *fileService) GetFile(w http.ResponseWriter, r *http.Request) {
	re := regexp.MustCompile(s.base + `/file/([^/]+)/(.*)`)
	str := r.URL.String()
	match := re.FindStringSubmatch(str)
	if len(match) != 3 {
		w.WriteHeader(400)
		w.Write([]byte(`Invalid path`))
		s.log.L().Error("minio get %s: invalid path", str)
		return
	}
	bucketName := match[1]
	objectName, err := url.QueryUnescape(match[2])
	if err != nil {
		s.log.L().Errorf("minio get %s: could not unescape (%s)", match[2], err.Error())
		return
	}

	object, err := s.mcli.GetObject(bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		s.log.L().Errorf("minio get %s/%s: %s", bucketName, objectName, err.Error())
		return
	}
	io.Copy(w, object)
	s.log.L().Infof("minio get %s/%s", bucketName, objectName)
}

func (s *fileService) UploadFile(w http.ResponseWriter, r *http.Request) {
	// TODO: choose optimal maxMemory value
	r.ParseMultipartForm(10 * MB)

	hv := phttp.ExtractHeaders(s.headersPass, r.Header) // extract specific HTTP headers
	data := mergeValues(r.MultipartForm, hv)            // merge them to multipart/form data

	type FileDescriptor struct {
		Name   string
		Size   int64
		Ext    string
		Header *multipart.FileHeader
	}

	var totalSize int64
	files := []FileDescriptor{}
	fileList := []string{}
	for _, fh := range r.MultipartForm.File {
		if len(fh) == 0 {
			continue
		}
		// TODO: support multiple similarly named files
		fname := fh[0].Filename
		ext := strings.Replace(filepath.Ext(fname), ".", "", -1)
		size := fh[0].Size
		totalSize += size
		files = append(files, FileDescriptor{Name: fname, Size: size, Ext: ext, Header: fh[0]})
		fileList = append(fileList, fname)
	}

	ErrorResponse := func(code int, msg string, str string) {
		w.WriteHeader(code)
		w.Write([]byte(fmt.Sprintf(`{"code":%d, "msg": "%s", "descr": "%s"}`, code, msg, str)))
		s.log.L().Error("minio post [%s]: %s", strings.Join(fileList, ","), str)
	}

	if len(files) == 0 {
		ErrorResponse(400, "no files", "No files in multipart/form-data")

		return
	}

	if data["bucket"] == nil && data["category"] == nil {
		ErrorResponse(400, "no bucket", "Required multipart field 'bucket' is missing")

		return
	}

	// check total size
	if totalSize > s.cfg.SizeLimit {
		ErrorResponse(400, "total size", fmt.Sprintf("Total file size %d is beyond limit on single upload (%d)", totalSize, s.cfg.SizeLimit))

		return
	}

	var fpaths []string

	for _, f := range files {
		// Extensions whitelist
		if _, found := s.cfg.AllowedExtMap[f.Ext]; !found && len(s.cfg.AllowedExt) > 0 {
			ErrorResponse(415, "bad ext", fmt.Sprintf("File extension '%s' is not allowed by config", f.Ext))

			return
		}
		// check file size
		if f.Size > s.cfg.SizeLimit {
			ErrorResponse(413, "file size", fmt.Sprintf("File '%s': size %d is beyond limit (%d)", f.Name, f.Size, s.cfg.SizeLimit))

			return
		}
		// read file
		file, err := f.Header.Open()
		if err != nil {
			ErrorResponse(400, "read error", fmt.Sprintf("Error reading file '%s': %s", f.Name, err.Error()))

			return
		}

		// store
		if data["category"] == nil {
			data["category"] = data["bucket"]
		}
		bucket := data["category"].(string)
		data["filename"] = f.Name
		data["filesize"] = f.Size
		data["fileext"] = f.Ext
		prefix, err := s.storeMetadata(s.cfg.Procedure, data)
		if err != nil {
			ErrorResponse(500, "db error", fmt.Sprintf("Error storing metadata on file '%s': %s", f.Name, err.Error()))

			return
		}

		err = s.minioUpload(bucket, prefix+f.Name, f.Header.Header.Get("Content-Type"), file, f.Size)
		if err != nil {
			ErrorResponse(500, "minio error", fmt.Sprintf("Error while storing file '%s' in minio: %s", f.Name, err.Error()))

			return
		}

		fpaths = append(fpaths, "/"+s.base+"/file/"+bucket+"/"+prefix+f.Name)
	}

	str := fmt.Sprintf(`{"file":"%s", "files":["%s"], "status":"ok"}`,
		fpaths[0],
		strings.Join(fpaths, `","`),
	)
	w.Header().Set("Content-Type", "text/json; charset=utf-8")
	w.Write([]byte(str))
}

func (s *fileService) minioUpload(bucket string, fname string, contentType string, f io.Reader, size int64) error {

	location := "us-east-1"

	err := s.mcli.MakeBucket(bucket, location)
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := s.mcli.BucketExists(bucket)
		if errBucketExists == nil && exists {
			fmt.Printf("We already own %s\n", bucket)
		} else {
			return err
		}
	} else {
		fmt.Printf("Successfully created %s\n", bucket)
	}

	n, err := s.mcli.PutObject(bucket, fname, f, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		s.log.L().Fatal(err)
	}
	fmt.Printf("%s: %d/%d bytes written\n", fname, n, size)

	return nil
}

func mergeValues(f *multipart.Form, hv []phttp.HeaderValue) map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range f.Value {
		m[k] = strings.Join(v, "\n")
	}
	for _, h := range hv {
		if h.Name != "" {
			m[h.Name] = h.Value
		}
	}
	return m
}

func (s *fileService) storeMetadata(proc string, data map[string]interface{}) (prefix string, err error) {
	var b []byte
	b, err = json.Marshal(data)
	if err != nil {
		return
	}
	query := "select * from " + proc + "($1)"
	rows, err := s.dbw.Query(query, string(b))
	if err != nil {
		return
	}
	defer rows.Close()

	var errMsg string
	for rows.Next() {
		err = rows.Scan(&prefix, &errMsg)
		if err != nil {
			return
		}
		if len(errMsg) > 0 {
			err = errors.New(errMsg)
		}
	}

	return
}
