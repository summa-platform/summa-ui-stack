package main

import (
	// "crypto/md5"
	// "encoding/json"
	// "fmt"
	"log"
	"net/http"
	// "strings"
	"time"

	_ "expvar" // /debug/vars
	// "io"
	// "html"
	"os"

	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)
import "runtime/debug"

func logHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("%s %s", r.Method, r.URL.Path)
		wlog := &LogResponseWritter{0, 0, w}
		// defer log.Printf("%v %s %s --> %v %v", r.Method, r.URL.Path, time.Since(start))
		defer func() {
			// log.Printf("%v %s %s --> %v %v", wlog.Status(), r.Method, r.URL.Path, time.Since(start), wlog.Size())
			log.Printf("%v %s %s --> %v in %v", wlog.Status(), r.Method, r.URL.Path, wlog.Size(), time.Since(start))
		}() // finished
		next.ServeHTTP(wlog, r)
	})
	// log response: https://medium.com/@matryer/the-http-handler-wrapper-technique-in-golang-updated-bc7fbcffa702
}

func debugHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer log.Printf("!!!!! DEBUG: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func noCacheHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, max-age=0, no-cache, no-store")
		next.ServeHTTP(w, r)
	})
}

func corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fmt.Println(r.Header["Origin"][0])
		if r.Method == "OPTIONS" {
			if hdrs, prs := r.Header["Origin"]; prs && len(hdrs) > 0 {
				w.Header().Set("Access-Control-Allow-Origin", r.Header["Origin"][0])
			}
			if hdrs, prs := r.Header["Access-Control-Allow-Headers"]; prs && len(hdrs) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", r.Header["Access-Control-Request-Headers"][0])
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.WriteHeader(http.StatusOK)
			return
		} else {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if hdrs, prs := r.Header["Origin"]; prs && len(hdrs) > 0 {
				w.Header().Set("Access-Control-Allow-Origin", r.Header["Origin"][0])
			}
			w.Header().Set("Access-Control-Expose-Headers", "Authorization, Content-Type")
		}
		next.ServeHTTP(w, r)
	})
}

func panicHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Println("Internal error:", err)
				fmt.Fprintln(os.Stderr, string(debug.Stack()))
				w.WriteHeader(http.StatusInternalServerError) // may cause multiple WriteHeader calls warning if already called by the next handler
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func ServeFile(path string, contentType string) http.Handler {
	if contentType == "" {
		contentType = "text/plain"
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".json" {
			contentType = "application/json"
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("internal error: %v", err)))
			return
		}
		w.Header()["Content-Type"] = []string{contentType}
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
}

func joinPath(base string, rel string) string {
	if path.IsAbs(rel) {
		return rel
	}
	if strings.HasPrefix(rel, "./") || strings.HasPrefix(rel, "../") {
		return rel
	}
	return filepath.Join(base, rel)
}

func runServer() {

	mux := http.NewServeMux()

	// mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
	// 	defer func() {
	// 		if err := recover(); err != nil {
	// 			w.WriteHeader(http.StatusInternalServerError)	// may cause multiple WriteHeader calls warning if already called by the next handler
	// 			log.Println(err)
	// 			// defer panic(err)
	// 			fmt.Println(string(debug.Stack()))
	// 		}
	// 	}()
	// 	panic("bac")
	// })

	// mux.Handle("/api/auth", http.StripPrefix("/api/auth", apiAuthenticate))
	mux.HandleFunc("/api/auth", apiAuthenticate)
	// mux.HandleFunc("/api/auth/token", apiGetAuthToken)
	mux.HandleFunc("/logout", logout)
	mux.HandleFunc("/api/logout", logout)
	// /login -> /login/
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login/", http.StatusTemporaryRedirect)
	})
	mux.Handle("/login/", loginAuthHandler(http.StripPrefix("/login", http.FileServer(http.Dir(config.LoginStaticPath)))))

	// nested mux pattern: https://stackoverflow.com/a/43380025
	mux.Handle("/api/users/checkPassword", http.StripPrefix("/api", http.HandlerFunc(apiUsers))) // bypass authentication
	mux.Handle("/api/auth/token", http.HandlerFunc(apiGetAuthCookieToken))                       // get token
	mux.Handle("/api/", http.StripPrefix("/api", apiMux()))

	mux.Handle("/favicon.ico", ServeFile(joinPath(config.StaticPath, config.ConfigPath), "")) // public access to config: sets /api/ path
	// http.Handle("/", http.FileServer(http.Dir("static")))
	// mux.Handle("/config.json", http.FileServer(http.Dir(config.StaticPath))) // public access to config: sets /api/ path
	mux.Handle("/config.json", ServeFile(joinPath(config.StaticPath, config.ConfigPath), "")) // public access to config: sets /api/ path
	mux.Handle("/", authHandler(http.FileServer(http.Dir(config.StaticPath))))

	// mux.Handle("/debug/vars", debugHandler(http.DefaultServeMux))
	mux.Handle("/debug/vars", http.DefaultServeMux)

	listenAddress := ":" + strconv.Itoa(config.Port)
	log.Fatal(http.ListenAndServe(listenAddress, corsHandler(noCacheHandler(logHandler(panicHandler(mux))))))
}
