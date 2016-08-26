package v8fetch

import "net/http"

// AddHeaders wraps Server and adds all of the provided headers to any
// request processed by it.  This can be used to copy cookies from a client
// request to all fetch calls during server-side rendering.
type AddHeaders struct {
	Server  http.Handler
	Headers http.Header
}

func (a AddHeaders) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for key, vals := range a.Headers {
		for _, val := range vals {
			r.Header.Add(key, val)
		}
	}
	a.Server.ServeHTTP(w, r)
}
