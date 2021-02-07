package files

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bhmj/pg-api/internal/pkg/config"
	"github.com/minio/minio-go"
)

type tUserKey int

const (
	userKey tUserKey = 0
	// MB is Megabyte
	MB = 1 << 20
)

type fileService struct {
	mcli *minio.Client
	cfg  *config.Minio
	dbw  *sql.DB
}

// FileService implements file service API
type FileService interface {
	UploadFile(userID int64, w http.ResponseWriter, r *http.Request)
	GetFile(userID int64, w http.ResponseWriter, r *http.Request)
}

// NewFileService returns new file service interface
func NewFileService(cfg *config.Minio, db *sql.DB) (FileService, error) {
	// Initialize minio client
	minioClient, err := minio.New(cfg.Host, cfg.AccessKey, cfg.SecretKey, cfg.UseSSL)
	if err != nil {
		return nil, err
	}
	return &fileService{
		cfg:  cfg,
		dbw:  db,
		mcli: minioClient,
	}, nil
}

// GetFile returns a file
func (s *fileService) GetFile(userID int64, w http.ResponseWriter, r *http.Request) {
	re := regexp.MustCompile(`/api/file/([^/]+)/(.*)`)
	str := r.URL.String()
	match := re.FindStringSubmatch(str)
	if len(match) != 3 {
		w.WriteHeader(400)
		w.Write([]byte(`Invalid path`))
		return
	}
	bucketName := match[1]
	objectName, err := url.QueryUnescape(match[2])
	if err != nil {
		fmt.Println(err)
		return
	}

	object, err := s.mcli.GetObject(bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	io.Copy(w, object)
}

func (s *fileService) UploadFile(userID int64, w http.ResponseWriter, r *http.Request) {
	//
	r.ParseMultipartForm(5 * MB)

	data := mergeValues(userID, r.MultipartForm)

	if data["category"] == nil {
		w.WriteHeader(400)
		w.Write([]byte(`Required field "category" is missing`))
		return
	}

	var fpath string
	var totalSize int64

	for _, fh := range r.MultipartForm.File {
		if len(fh) == 0 {
			continue
		}
		// Extensions whitelist
		ext := filepath.Ext(fh[0].Filename)
		ext = strings.Replace(ext, ".", "", -1)
		if len(s.cfg.AllowedExt) > 0 {
			match := false
			for _, x := range s.cfg.AllowedExt {
				if ext == x {
					match = true
					break
				}
			}
			if !match {
				w.WriteHeader(415)
				w.Write([]byte(`{"code":415, "msg":"bad ext", "descr":"File extention is not in white list"}`))
				return
			}
		}
		// check file size
		if fh[0].Size > s.cfg.SizeLimit {
			w.WriteHeader(413)
			w.Write([]byte(`{"code":413, "msg": "file size", "descr": "File size is beyond limit"}`))
			return
		}
		totalSize += fh[0].Size
		// check total files' size
		if totalSize > s.cfg.SizeLimit {
			w.WriteHeader(413)
			w.Write([]byte(`{"code":413, "msg": "total size", "descr": "Total file size is beyond limit"}`))
			return
		}
		// read file
		f, err := fh[0].Open()
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte(`Invalid file in multipart/form-data`))
			return
		}

		// store
		cat := data["category"].(string)
		data["filename"] = fh[0].Filename
		data["filesize"] = fh[0].Size
		data["fileext"] = filepath.Ext(fh[0].Filename)
		prefix, err := s.storeMetadata(s.cfg.Procedure, data)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}

		err = s.minioUpload(cat, prefix+fh[0].Filename, fh[0].Header.Get("Content-Type"), f, fh[0].Size)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(`File storage error: ` + err.Error()))
			return
		}

		fpath = "/api/file/" + cat + "/" + prefix + fh[0].Filename
	}

	w.Header().Set("Content-Type", "text/json; charset=utf-8")
	w.Write([]byte(`{"file":"` + strings.Replace(fpath, `"`, `\"`, -1) + `", "status":"ok"}`))
}

func (s *fileService) minioUpload(cat string, fname string, contentType string, f io.Reader, size int64) error {

	// Make a new bucket called mymusic.
	bucketName := cat
	location := "us-east-1"

	err := s.mcli.MakeBucket(bucketName, location)
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := s.mcli.BucketExists(bucketName)
		if errBucketExists == nil && exists {
			fmt.Printf("We already own %s\n", bucketName)
		} else {
			return err
		}
	} else {
		fmt.Printf("Successfully created %s\n", bucketName)
	}

	n, err := s.mcli.PutObject(bucketName, fname, f, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("%s: %d/%d bytes written\n", fname, n, size)

	return nil
}

func mergeValues(userID int64, f *multipart.Form) map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range f.Value {
		m[k] = strings.Join(v, "\n")
	}
	m["user_id"] = userID
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
