package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bhmj/jsonslice"
	"github.com/bhmj/pg-api/internal/pkg/config"
)

func (s *service) enhanceData(body []byte, enhance []config.Enhance, timeout time.Duration) []byte {

	var obj interface{}
	err := json.Unmarshal(body, &obj)
	if err != nil {
		return body
	}

	rx, _ := regexp.Compile(`%\d+`)
	re, _ := regexp.Compile(`{(\$.+?)}`) // Regexp for keys of `{$key}` view
	vals := make(map[string][]byte, 5)   // Map of values from the body, indexed by keys

next:
	for _, enh := range enhance {

		startTime := time.Now() // metric

		tmp, _ := json.Marshal(obj)

		// List of keys in current URL: [["{$key}", "$key"], ...]
		keys := re.FindAllStringSubmatch(enh.URL, -1)

		// Replace all keys in URL with the corresponding values from the body
		for i := 0; i < len(keys); i++ {
			key := keys[i][1] // "$keyi"
			if _, ok := vals[key]; !ok {
				val, err := jsonslice.Get(tmp, key) // Get value from the body by key
				if err != nil {
					s.log.L().Errorf("jsonslice fail: %s: %s on %s", err.Error(), key, string(tmp), key)
					continue next
				}
				if val[0] == '"' { // If val is in double quotes
					val = val[1 : len(val)-1] // Get rid of quotes
				}
				vals[key] = val

			}
			enh.URL = strings.Replace(enh.URL, keys[i][0], string(vals[key]), -1)
		}

		if enh.Condition != "" {
			cond := "$[?(" + enh.Condition + ")]"
			result, err := jsonslice.Get([]byte("["+string(tmp)+"]"), cond)
			if err != nil {
				s.log.L().Errorf("jsonslice condition fail: %s: %s on %s", err.Error(), cond, string(tmp))
				continue
			}
			if string(result) == "[]" {
				continue
			}
		}

		// Extract draft of external service name from enh.URL
		submatches := regexpMap["extServiceName"].FindAllStringSubmatch(enh.URL, -1)
		// http://domain.com/api/v1/some/service?param=foo -> api/v1/some/service
		extServiceName := "external"
		if submatches != nil {
			extServiceName := submatches[0][1]
			// Split extServiceName into substrings of [:word:] class symbols (a-zA-Z0-9_)
			// api/v1/some/service -> ["api","v1","some","service"]
			substrings := regexpMap["splitExtServiceName"].FindAllString(extServiceName, -1)
			// Finally concatenate substrings with "_" separator into extServiceName
			// ["api","v1","some","service"] -> api_v1_some_service
			extServiceName = strings.Join(substrings, "_")
		}
		data, flds, err := s.queryExternal(enh, tmp, timeout)
		if err != nil {
			s.log.L().Errorf("queryExternal: %s", err.Error())
			continue
		}
		if s.cfg.LogLevel >= 2 { // warnings, verbose
			s.log.L().Infof("queryExternal result: %s", string(data))
		}

		for _, dst := range enh.TransferFields {
			for _, match := range rx.FindAllString(dst.From, -1) {
				idx, _ := strconv.Atoi(strings.Replace(match, "%", "", -1))
				dst.From = strings.Replace(dst.From, match, fmt.Sprintf("%v", flds[enh.ForwardFields[idx-1]]), -1)
			}
			v, err := jsonslice.Get(data, dst.From)
			if err != nil {
				s.log.L().Errorf("jsonslice(\"%s\") : %s", dst.From, err.Error())
				continue
			}
			var value interface{}
			err = json.Unmarshal(v, &value)
			if err != nil {
				s.log.L().Errorf("json.Unmarshal(\"%s\") : %s", string(v), err.Error())
				continue
			}

			vobj := obj.(map[string]interface{})
			vobj[dst.To] = value
		}

		s.metrics.Score(s.method, s.vpath, extServiceName, startTime, nil)
	}

	body, _ = json.Marshal(obj)

	if err != nil {
		s.log.L().Error(err)
		return body
	}

	return body
}
