package main

import (
	// "fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func reverseProxyDirector(req *http.Request) {
	req.Host = req.URL.Hostname()
}

var videoProxy = &httputil.ReverseProxy{Director: reverseProxyDirector}

func apiVideo(w http.ResponseWriter, r *http.Request) {

	get := r.Method == "GET"

	if !get {
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if parts[0] == "" {
		parts = parts[1:]
	}

	p := 0
	nparts := len(parts)

	if nparts <= p || parts[p] != "video" {
		panic("invalid path prefix")
	}

	p++

	// video/id/<rest-url>

	if nparts > p {

		origin := parts[p]
		p++

		base, prs := origins[origin]
		if !prs {
			// if origin not found, try to refresh from database first
			dbGetOrigins()
			base, prs = origins[origin]
		}

		if !prs {
			apiLogf(r, "media-item origin id %v not in database", origin)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if base, prs = origins[origin]; prs {

			// prepare URL string
			relURL := "/" + strings.Join(parts[p:], "/")
			// remove hard-coded /video-chunks/ prefix
			if strings.HasPrefix(relURL, "/video-chunks/") {
				relURL = strings.TrimPrefix(relURL, "/video-chunks")
			}
			fullURL := strings.TrimSuffix(base.videoURL, "/") + "/" + strings.TrimPrefix(relURL, "/") // join base URL with relative URL

			videoURL, err := url.Parse(fullURL)
			if err != nil {
				apiLogf(r, "unable to parse (origin %v) url %v: %v", origin, fullURL, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			r.URL = videoURL

			videoProxy.ServeHTTP(w, r)

			if w.(*ResponseWriterWithHandledStatus).Status() != 200 {
				dbGetOrigins() // reload origins from db to retrieve possible updates
			}

			return

		} else {
			apiLogf(r, "media-item origin id %v not in database", origin)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
