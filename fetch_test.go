package v8fetch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/augustoroman/v8"
)

func TestLocalFetch(t *testing.T) {
	ctx := v8.NewIsolate().NewContext()
	if err := Inject(ctx, testServer); err != nil {
		t.Fatal(err)
	}

	resp, err := parseResponse(runPromise(ctx, `fetch("/foo")`))
	if err != nil {
		t.Fatal(err)
	}

	expected := response{
		options: options{
			Url:    "/foo",
			Method: "GET",
			Headers: http.Header{
				"Content-Type": []string{"text/plain; charset=utf-8"},
				"X-Answer":     []string{"42"},
			},
			Body: "got foo",
		},
		Status:     200,
		StatusText: "",
		Errors:     []error{},
	}

	if !reflect.DeepEqual(resp, expected) {
		t.Errorf("Wrong result:\nExp: %#v\nGot: %#v", expected, resp)
	}

	// Try bad URL
	resp, err = parseResponse(runPromise(ctx, `fetch("/404")`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 404 {
		t.Errorf("Expected 404, got status code %d\n%#v", resp.Status, resp)
	}
}

func TestRemoteFetch(t *testing.T) {
	ctx := v8.NewIsolate().NewContext()
	if err := Inject(ctx, nil); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(testServer)
	defer server.Close()

	resp, err := parseResponse(runPromise(ctx, fmt.Sprintf(
		`fetch("%s/bar")`, server.URL)))
	if err != nil {
		t.Fatal(err)
	}

	expected := `got bar`
	if resp.Body != expected {
		t.Errorf("Expected %q, got %q\n%#v", expected, resp.Body, resp)
	}

	//
	resp, err = parseResponse(runPromise(ctx, fmt.Sprintf(
		`fetch("%s/404")`, server.URL)))
	if err != nil {
		t.Fatal(err)
	}

	if resp.Status != 404 {
		t.Errorf("Expected 404, got status code %d\n%#v", resp.Status, resp)
	}
}

func TestHeaderOptions(t *testing.T) {
	server := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("extra") != "data" {
			t.Errorf("missing external headers: %q", r.Header)
		}
		w.Header().Set("x-answer", r.Header.Get("x-favorite-number"))
		fmt.Fprintf(w, "got foo")
	})
	ctx := v8.NewIsolate().NewContext()
	Inject(ctx, server)
	resp, err := parseResponse(runPromise(ctx, `
		fetch("/foo", {headers:{extra:"data","x-favorite-number":"42"}})
	`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Headers.Get("x-answer") != "42" {
		t.Errorf("Wrong response headers: %q", resp.Headers)
	}
}

var testServer = http.NewServeMux()

func init() {
	testServer.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-answer", "42")
		fmt.Fprintf(w, "got foo")
	})
	testServer.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "got bar")
	})
}

func runPromise(ctx *v8.Context, promiseCode string) (*v8.Value, error) {
	result := make(chan *v8.Value, 1)
	ctx.Global().Set("sendToGo", ctx.Bind("sendToGo",
		func(in v8.CallbackArgs) (*v8.Value, error) {
			result <- in.Arg(0)
			return nil, nil
		}))
	jsCode := promiseCode + ".then(sendToGo)"
	_, err := ctx.Eval(jsCode, "test.js")
	if err != nil {
		return nil, fmt.Errorf("Eval error: %v\nCode: %s", err, jsCode)
	}

	select {
	case res := <-result:
		return res, nil
	case <-time.After(time.Second / 10):
		return nil, fmt.Errorf("Timed out running code:\n%s", jsCode)
	}
}

func parseResponse(val *v8.Value, err error) (response, error) {
	if err != nil {
		return response{}, err
	}
	data, err := json.Marshal(val)
	if err != nil {
		return response{}, err
	}
	var resp response
	err = json.Unmarshal(data, &resp)
	return resp, err
}
