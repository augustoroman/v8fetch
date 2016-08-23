// Package v8fetch implements a fetch polyfill for the v8 engine embedded in Go.
//
// Basic usage looks like:
//      ctx := v8.NewIsolate().NewContext()
//      v8fetch.Inject(ctx, my_http_handler)
//      _, err := ctx.Eval(`
//            fetch("http://www.example.com/").then(process_response);
//            fetch("/local/mux").then(process_response);
//        `, "mycode.js")
//
// This code is based off of https://github.com/olebedev/go-duktape-fetch/ which
// implements the fetch polyfill for the duktape JS engine.
package v8fetch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/augustoroman/v8"
	"github.com/augustoroman/v8fetch/internal/data"
)

// Generate the embedded javascript code:
//go:generate npm install
//go:generate node_modules/.bin/webpack
//go:generate go-bindata -o internal/data/data.go -pkg=data dist/bundle.js

// Load the embedded javascript shim from the assets package.
var bundle = string(data.MustAsset("dist/bundle.js"))

// Inject inserts the fetch polyfill into ctx.  The server parameter may be non-
// nil to support relative URLs that have no host (e.g. /foo/bar instead of
// https://host.com/foo/bar).  If server is nil, then such relative URLs will
// always fail.  The fetch polyfill only supports http and https schemes.
func Inject(ctx *v8.Context, server http.Handler) error {
	const get_fetch = `;(function() { return fetch; })()`
	fetch, err := ctx.Eval(bundle+get_fetch, "fetch-bundle.js")
	if err != nil {
		return fmt.Errorf("v8fetch injection failed: %v", err)
	}
	fetcher := syncFetcher{ctx, server}
	return fetch.Set("goFetchSync", ctx.Bind("goFetchSync", fetcher.FetchSync))
}

type syncFetcher struct {
	ctx   *v8.Context
	local http.Handler
}

func (s syncFetcher) FetchSync(l v8.Loc, args ...*v8.Value) (*v8.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Expected 2 args (url, options), got %d.", len(args))
	}
	url := args[0].String()
	opts := options{
		Method:  "GET",
		Headers: http.Header{},
	}
	if err := json.Unmarshal([]byte(args[1].String()), &opts); err != nil {
		return nil, fmt.Errorf("Cannot decode JSON options: %v", err)
	}

	var resp response
	if strings.HasPrefix(url, "http") || strings.HasPrefix(url, "//") {
		resp = fetchHttp(url, opts)
	} else if strings.HasPrefix(url, "/") {
		resp = fetchHandlerFunc(s.local, url, opts)
	} else {
		return nil, fmt.Errorf(
			"v8fetch only supports http(s) or local (relative) URIs: %s", url)
	}

	return s.ctx.Create(resp)
}

type options struct {
	Url     string      `json:"url"`
	Method  string      `json:"method"`
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
}

type response struct {
	options
	Status     int     `json:"status"`
	StatusText string  `json:"statusText,omitempty"`
	Errors     []error `json:"errors"`
}

func fetchHttp(url string, opts options) response {
	result := response{options: opts, Status: http.StatusInternalServerError}

	var body io.Reader
	if opts.Method == "POST" || opts.Method == "PUT" || opts.Method == "PATCH" {
		body = strings.NewReader(opts.Body)
	}
	req, err := http.NewRequest(opts.Method, url, body)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}
	req.Header = opts.Headers

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		result.Errors = append(result.Errors, err)
	}
	if resp != nil {
		defer resp.Body.Close()
		result.Status = resp.StatusCode
		result.StatusText = resp.Status
		result.Headers = resp.Header
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			result.Errors = append(result.Errors, err)
		}
		result.Body = string(body)
	}

	return result
}

func fetchHandlerFunc(server http.Handler, url string, opts options) response {
	result := response{
		options: opts,
		Status:  http.StatusInternalServerError,
		Errors:  []error{},
	}

	if server == nil {
		result.Errors = append(result.Errors, errors.New("`http.Handler` isn't set yet"))
		return result
	}

	b := bytes.NewBufferString(opts.Body)
	res := httptest.NewRecorder()
	req, err := http.NewRequest(opts.Method, url, b)

	if err != nil {
		result.Errors = []error{err}
		return result
	}

	req.Header = opts.Headers
	req.Header.Set("X-Forwarded-For", "<local>")
	server.ServeHTTP(res, req)
	result.Status = res.Code
	result.Headers = res.Header()
	result.Body = res.Body.String()
	return result
}
