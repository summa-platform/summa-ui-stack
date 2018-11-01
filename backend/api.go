package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func apiLogf(r *http.Request, format string, v ...interface{}) {
	log.Printf("%s %s %v", r.Method, r.RequestURI, fmt.Sprintf(format, v...))
}

func apiLogln(r *http.Request, v ...interface{}) {
	log.Printf("%s %s %v", r.Method, r.RequestURI, fmt.Sprintln(v...))
}

func pretty(data interface{}) {
	out, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(out))
}

type ResponseWriterWithHandledStatus struct {
	http.ResponseWriter
	handled bool
	status  int
	size    int
}

func (w *ResponseWriterWithHandledStatus) Handled() bool {
	return w.handled
}

func (w *ResponseWriterWithHandledStatus) Status() int {
	return w.status
}

func (w *ResponseWriterWithHandledStatus) Size() int {
	return w.size
}

func (w *ResponseWriterWithHandledStatus) WriteHeader(status int) {
	w.handled = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *ResponseWriterWithHandledStatus) Write(b []byte) (written int, err error) {
	if !w.handled {
		w.handled = true
	}
	written, err = w.ResponseWriter.Write(b)
	w.size += written
	return
}

func unhandledHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wws := &ResponseWriterWithHandledStatus{ResponseWriter: w}
		next.ServeHTTP(wws, r)
		if wws.handled {
			return
		}
		// unhandled
		wws.WriteHeader(http.StatusNotFound)
	})
}

func apiMux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/feeds", apiFeeds)
	mux.HandleFunc("/feeds/", apiFeeds)
	mux.HandleFunc("/feedGroups", apiFeedGroups)
	mux.HandleFunc("/feedGroups/", apiFeedGroups)
	mux.HandleFunc("/users", apiUsers)
	mux.HandleFunc("/users/", apiUsers)
	mux.HandleFunc("/namedEntities", apiNamedEntities)
	mux.HandleFunc("/namedEntities/", apiNamedEntities)
	mux.HandleFunc("/queries", apiQueries)
	mux.HandleFunc("/queries/", apiQueries)
	mux.HandleFunc("/mediaItems", apiMediaItems)
	mux.HandleFunc("/mediaItems/", apiMediaItems)
	mux.HandleFunc("/feedback", apiReports)
	mux.HandleFunc("/feedback/", apiReports)
	mux.HandleFunc("/video/", apiVideo)
	mux.HandleFunc("/locations", apiGeolocation)
	mux.HandleFunc("/locations/", apiGeolocation)

	handler := unhandledHandler(mux)

	// return headerAuthHandler(handler)	// production - header only auth
	return authHandler(handler) // development - header or cookie auth
	// return handler // local development - no auth
}
