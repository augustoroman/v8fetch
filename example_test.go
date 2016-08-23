package v8fetch_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/augustoroman/v8"
	"github.com/augustoroman/v8/v8console"
	"github.com/augustoroman/v8fetch"
)

func Example_basic() {
	ctx := v8.NewIsolate().NewContext()
	v8console.Config{"", os.Stdout, os.Stderr, true}.Inject(ctx)
	v8fetch.Inject(ctx, nil)

	ctx.Eval(`
        fetch('https://golang.org/')
            .then(r => console.log(r.body.slice(0, 15)));
        `, "code.js")

	// Output:
	// <!DOCTYPE html>
}

func Example_localServer() {
	// If you are running a binary that is also an http server, you probably
	// have an http.Handler that is routing & managing you requests.  That's
	// "local" in this example:
	local := http.NewServeMux()
	local.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "foo from the local server")
	})

	// But you might also need to fetch results from somewhere else, like
	// S3 or a CDN or something.  In this example, it's the remote server.
	remote := http.NewServeMux()
	remote.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "bar from afar")
	})
	server := httptest.NewServer(remote)
	defer server.Close()

	ctx := v8.NewIsolate().NewContext()
	v8console.Config{"", os.Stdout, os.Stderr, false}.Inject(ctx)
	v8fetch.Inject(ctx, local) // local may be nil if there's no local server

	ctx.Eval(fmt.Sprintf(`
        fetch("/foo")
            .then(r => console.log('Local:', r.body, '('+r.status+')'));

        fetch("%s/bar")
            .then(r => console.log('Remote:', r.body, '('+r.status+')'));

        fetch("/no-such-page")
            .then(r => console.log('Local (missing):', r.body, '('+r.status+')'));
    `, server.URL), "mycode.js")

	// Output:
	// Local: foo from the local server (200)
	// Remote: bar from afar (200)
	// Local (missing): 404 page not found
	//  (404)
}
