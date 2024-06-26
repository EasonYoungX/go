// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	dateRE     = regexp.MustCompile(`(\d\d\d\d-\d\d-\d\d)[.]json$`)
	dateFormat = "2006-01-02"
	// TODO(rfindley): use dateFormat throughout.
)

// uploadReportDate returns the date component of the upload file name, or "" if the
// date was unmatched.
func (u *Uploader) uploadReportDate(fname string) time.Time {
	match := dateRE.FindStringSubmatch(fname)
	if match == nil || len(match) < 2 {
		u.logger.Printf("malformed report name: missing date: %q", filepath.Base(fname))
		return time.Time{}
	}
	d, err := time.Parse(dateFormat, match[1])
	if err != nil {
		u.logger.Printf("malformed report name: bad date: %q", filepath.Base(fname))
		return time.Time{}
	}
	return d
}

func (u *Uploader) uploadReport(fname string) {
	thisInstant := u.startTime
	// TODO(rfindley): use uploadReportDate here, once we've done a gopls release.

	// first make sure it is not in the future
	today := thisInstant.Format("2006-01-02")
	match := dateRE.FindStringSubmatch(fname)
	if match == nil || len(match) < 2 {
		u.logger.Printf("report name seemed to have no date %q", filepath.Base(fname))
	} else if match[1] > today {
		u.logger.Printf("report %q is later than today %s", filepath.Base(fname), today)
		return // report is in the future, which shouldn't happen
	}
	buf, err := os.ReadFile(fname)
	if err != nil {
		u.logger.Printf("%v reading %s", err, fname)
		return
	}
	if u.uploadReportContents(fname, buf) {
		// anything left to do?
	}
}

// try to upload the report, 'true' if successful
func (u *Uploader) uploadReportContents(fname string, buf []byte) bool {
	b := bytes.NewReader(buf)
	fdate := strings.TrimSuffix(filepath.Base(fname), ".json")
	fdate = fdate[len(fdate)-len("2006-01-02"):]
	server := u.uploadServerURL + "/" + fdate

	resp, err := http.Post(server, "application/json", b)
	if err != nil {
		u.logger.Printf("error on Post: %v %q for %q", err, server, fname)
		return false
	}
	// hope for a 200, remove file on a 4xx, otherwise it will be retried by another process
	if resp.StatusCode != 200 {
		u.logger.Printf("resp error on upload %q: %v for %q %q [%+v]", server, resp.Status, fname, fdate, resp)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			err := os.Remove(fname)
			if err == nil {
				u.logger.Printf("removed")
			} else {
				u.logger.Printf("error removing: %v", err)
			}
		}
		return false
	}
	// put a copy in the uploaded directory
	newname := filepath.Join(u.dir.UploadDir(), fdate+".json")
	if err := os.WriteFile(newname, buf, 0644); err == nil {
		os.Remove(fname) // if it exists
	}
	u.logger.Printf("uploaded %s to %q", fdate+".json", server)
	return true
}
