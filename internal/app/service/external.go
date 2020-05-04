package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/bhmj/jsonslice"
	"github.com/bhmj/pg-api/internal/pkg/config"
)

func (s *service) queryExternal(enh config.Enhance, sourceJSON []byte, timeout time.Duration) (response []byte, flds map[string]interface{}, err error) {

	var req *http.Request
	var body []byte
	flds = make(map[string]interface{})

	arrayMode := false
	if len(enh.ForwardFields) > 0 && enh.ForwardFields[0] == "[]" {
		arrayMode = true
	}

	nonNilCount := 0 // Counter of fields in enh.IncomingFields with non-nil value
	for i, e := range enh.IncomingFields {
		var value interface{}

		switch string(e[0]) {
		case "$":
			var v []byte
			v, err = jsonslice.Get(sourceJSON, e)
			if err == nil {
				err = json.Unmarshal(v, &value)
				if err != nil {
					return
				}
				if reflect.TypeOf(value).Name() == "float64" {
					v := value.(float64)
					if v == math.Trunc(v) {
						value = int64(v)
					}
				}
			}
		case "~":
			switch e {
			case "~null":
				value = nil
			case "~true":
				value = true
			case "~false":
				value = false
			}
		default:
			value = e
		}
		if err == nil {
			nonNilCount++
			flds[enh.ForwardFields[i]] = value
		}
	}
	// If all fields in enh.IncomingFields are nil and enh.IncomingFields isn't empty
	if nonNilCount == 0 && len(enh.IncomingFields) > 0 {
		err = errors.New("no Enhance.IncomingFields filled")
		return
	}

	if enh.Method == "POST" {
		if arrayMode {
			body, err = json.Marshal([]interface{}{flds[enh.ForwardFields[0]]})
		} else {
			body, err = json.Marshal(flds)
		}
		if err != nil {
			return
		}
		req, err = http.NewRequest(enh.Method, enh.URL, bytes.NewReader(body))
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Content-Length", strconv.Itoa(len(body)))
	} else {
		req, err = http.NewRequest(enh.Method, enh.URL, nil)
		if err != nil {
			return
		}
		q := req.URL.Query()
		for key, value := range flds {
			q.Add(key, fmt.Sprintf("%v", value))
		}
		req.URL.RawQuery = q.Encode()
	}

	var resp *http.Response

	client := &http.Client{Timeout: timeout}
	resp, err = client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("%s: status %d", enh.URL, resp.StatusCode)
		return
	}

	body, err = ioutil.ReadAll(resp.Body)

	return body, flds, nil
}
