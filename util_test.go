package v8fetch_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/augustoroman/v8/v8console"

	"github.com/augustoroman/v8"
	"github.com/augustoroman/v8fetch"
)

func TestAddCookieHeader(t *testing.T) {
	ref, _ := http.NewRequest("X", "/", nil)
	ref.AddCookie(&http.Cookie{Name: "test", Value: "it works!"})

	server := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("test")
		if err != nil {
			t.Fatalf("Missing cookie 'test': %v", err)
		}
		if c.Value != "it works!" {
			t.Errorf("Wrong value for test cookie: %q", c.Value)
		}
		w.WriteHeader(200)
	})

	var logs bytes.Buffer
	ctx := v8.NewIsolate().NewContext()
	v8fetch.Inject(ctx, v8fetch.AddCookieHeader{server, ref})
	v8console.Config{"", &logs, &logs, false}.Inject(ctx)
	_, err := ctx.Eval(`
            fetch('/foo').then(r => { console.log(r.status) })
        `, "test.js")
	if err != nil {
		t.Fatal(err)
	}

	if logs.String() != "200\n" {
		t.Errorf("Didn't get the expected response code: %s", logs.String)
	}
}
