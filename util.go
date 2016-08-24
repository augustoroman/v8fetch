package v8fetch

import "net/http"

// AddCookieHeader wraps Server and adds all of the cookie headers from
// Reference into any request processed by it.  This can be used to copy cookies
// from a client request to all fetch calls during server-side rendering
type AddCookieHeader struct {
	Server    http.Handler
	Reference *http.Request
}

func (a AddCookieHeader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, val := range a.Reference.Header["Cookie"] {
		r.Header.Add("Cookie", val)
	}
	a.Server.ServeHTTP(w, r)
}
