package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// type AuthClaims map[string]interface{}
type AuthClaims = jwt.MapClaims

type AuthInfo struct {
	ID    string
	Admin bool
	// Claims jwt.MapClaims
	Claims AuthClaims
}

var privateKey []byte

func loadPrivateKey(keyfile string) {
	privateKey, _ = ioutil.ReadFile(keyfile)
}

const authTokenExpireSeconds = 3600
const authTokenRefreshSeconds = 600

//
// JWT parse & generate
//

func parseJWToken(tokenString string) (jwt.MapClaims, error) {

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return privateKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("Token not valid")
}

/*
func generateToken(claims map[string]interface{}) string {
	c := make(jwt.MapClaims, len(claims)+1)
	for k, v := range claims {
		c[k] = v
	}
	c["exp"] = time.Now().Unix() + 3600 // 1h

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	// token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
	// 	// "foo": "bar",
	// 	"exp": time.Now().Unix() + 36000,
	// 	// "nbf": time.Date()
	// 	// "exp": time.Now().Unix() - 1,
	// 	// "exp": time.Now().Add(time.Hour * 72).Unix(),
	// 	// "nbf": time.Now().Unix() + 36000,
	// })

	tokenString, _ := token.SignedString(privateKey)

	return tokenString
}
*/

func verifyJWTokenWithClaims(tokenString string) (ok bool, claims jwt.MapClaims) {
	claims, err := parseJWToken(tokenString)
	if err != nil {
		log.Print("JWT token error:", err)
		return false, nil
	}
	if claims == nil {
		log.Print("JWT token error: missing claims object")
		return false, nil
	}
	// if claims["user"] == nil && claims["email"] == nil {
	// if claims["id"] == nil {
	// 	log.Print("JWT token error: id not set")
	// 	return false, nil
	// }
	return true, claims
}

func verifyJWToken(tokenString string) (ok bool) {
	ok, _ = verifyJWTokenWithClaims(tokenString)
	return ok
}

func generateJWTWithClaims(claims AuthClaims) string {
	if claims == nil {
	}
	claims["iat"] = float64(time.Now().Unix()) // time created / issued at
	claims["exp"] = float64(time.Now().Unix() + authTokenExpireSeconds)
	claims["upd"] = float64(time.Now().Unix() + authTokenRefreshSeconds) // refresh hardcoded into token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(privateKey)
	return tokenString
}

func generateJWT(id string) string {
	return generateJWTWithClaims(AuthClaims{"id": id})
}

func shouldRefreshJWT(claims AuthClaims) bool {
	if claims["iat"] != nil {
		return claims["iat"].(float64)+authTokenRefreshSeconds <= float64(time.Now().Unix()) // dynamically calculate update time
		// return claims["iat"].(int64)+authTokenRefreshSeconds <= time.Now().Unix() // dynamically calculate update time
	}
	if claims["upd"] == nil {
		return true
	}
	// return claims["upd"].(int64) <= time.Now().Unix() // use into JWT hardcoded update time
	return claims["upd"].(float64) <= float64(time.Now().Unix())
}

//
// Auth Header & Cookie
//

func parseAuthHeaderItem(headerItem string) AuthClaims {
	parts := strings.SplitN(headerItem, " ", 2)
	if parts[0] == "Bearer" {
		if claims, err := parseJWToken(parts[1]); err == nil && claims != nil {
			return claims
		}
	}
	return nil
}

func parseAuthHeader(header []string) AuthClaims {
	for _, item := range header {
		if claims := parseAuthHeaderItem(item); claims != nil {
			return claims
		}
	}
	return nil
}

func parseAuthCookie(cookie *http.Cookie) AuthClaims {
	if cookie.Name == "auth" {
		if claims, err := parseJWToken(cookie.Value); err == nil && claims != nil {
			return claims
		}
	}
	return nil
}

//
// Request level auth Header & Cookie functions
//

func requestAuthHeaderClaims(r *http.Request) AuthClaims {
	authHdr, prs := r.Header["Authorization"]
	if prs {
		if claims := parseAuthHeader(authHdr); claims != nil {
			return claims
		}
	}
	return nil
}

func requestAuthCookieClaims(r *http.Request) AuthClaims {
	for _, cookie := range r.Cookies() {
		if claims := parseAuthCookie(cookie); claims != nil {
			return claims
		}
	}
	return nil
}

// --- not used for now
func requestAuthClaims(r *http.Request) AuthClaims {
	// first, try to extract from header
	if claims := requestAuthHeaderClaims(r); claims != nil {
		return claims
	}
	// second, try to extract from cookie
	if claims := requestAuthCookieClaims(r); claims != nil {
		return claims
	}
	return nil
}

func requestVerifyAuthHeader(r *http.Request) bool {
	authHdr, prs := r.Header["Authorization"]
	if prs {
		if claims := parseAuthHeader(authHdr); claims != nil {
			return true
		}
	}
	return false
}

func requestVerifyAuthCookie(r *http.Request) bool {
	if claims := requestAuthCookieClaims(r); claims != nil {
		return true
	}
	return false
}

func requestAuthCookieToken(r *http.Request) *string {
	for _, cookie := range r.Cookies() {
		if claims := parseAuthCookie(cookie); claims != nil {
			return &cookie.Value
		}
	}
	return nil
}

func responseSetAuthCookie(tokenString string, w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "auth", Value: tokenString, Expires: time.Now().Add(authTokenExpireSeconds * time.Second), Path: "/"})
}

func responseDeleteAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "auth", Value: "", Expires: time.Unix(0, 0), Path: "/"})
}

func responseSetAuthHeader(tokenString string, w http.ResponseWriter) {
	w.Header()["Authorization"] = []string{"Bearer " + tokenString}
}

//
// API functions
//

func apiGetAuthCookieToken(w http.ResponseWriter, r *http.Request) {
	if tokenString := requestAuthCookieToken(r); tokenString != nil {
		w.Header()["Content-Type"] = []string{"text/plain; charset=utf-8"}
		// w.WriteHeader(http.StatusOK)	// not needed by default
		fmt.Fprintf(w, *tokenString)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
	}
}

func apiAuthenticate(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Authenticate remote addr:", r.RemoteAddr)

	var credentials struct {
		User     string `json:"user"`
		Email    string `json:"email"`
		Password string `json:"pwd"`
	}

	var redirect string

	// fmt.Println(r.Header)
	// fmt.Println(r.Method)
	// fmt.Println(r.Header["Content-Type"][0] == "application/x-www-form-urlencoded")

	loginForm := r.Method == "POST" && r.Header["Content-Type"][0] == "application/x-www-form-urlencoded"

	if loginForm {
		r.ParseForm()
		credentials.User = r.PostFormValue("user")
		credentials.Email = r.PostFormValue("email")
		credentials.Password = r.PostFormValue("password")
		redirect = r.PostFormValue("r")
	} else {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			fmt.Println("error", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
		}
	}

	validLogin, claims := verifyUserLogin(credentials.Email, credentials.Password)
	// TODO: check user/pwd here
	// validLogin := credentials.User == "admin" && credentials.Password == "admin"
	fmt.Println("loginform", loginForm)
	fmt.Println("valid", validLogin)

	if validLogin {
		fmt.Println("authenticate OK")
	} else if loginForm {
		fmt.Println("authenticate login form fail")
		http.Redirect(w, r, "/login/?status=fail", http.StatusTemporaryRedirect)
		return
	} else {
		fmt.Println("authenticate FAIL")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// if checkAuth(r) {
	// 	// TODO: write current token
	// 	w.WriteHeader(200)
	// 	return
	// }

	// https://stackoverflow.com/questions/30341588/how-to-parse-a-complicated-json-with-go-unmarshal
	// https://en.wikipedia.org/wiki/Digest_access_authentication

	tokenString := generateJWTWithClaims(claims)

	// tokenString := generateToken()

	// claims, err := parseJWToken(tokenString)
	// fmt.Println(claims, err)
	if verifyJWToken(tokenString) {
		fmt.Println("auth ok")
	}

	byt := []byte(`asdf`)
	// fmt.Printf(fmt.Sprintf("%x", md5.Sum(byt)))
	fmt.Sprintf("%x", md5.Sum(byt))

	w.Header()["Content-Type"] = []string{"text/plain; charset=utf-8"}
	// set cookie
	responseSetAuthCookie(tokenString, w)
	// set header
	responseSetAuthHeader(tokenString, w)

	if loginForm {
		if len(redirect) == 0 {
			redirect = "/"
		} else {
			redirect, _ = url.QueryUnescape(redirect)
			redirect = "/" + redirect
			// fmt.Println(redirect)
		}
		// fmt.Println("REDIRECT:", redirect)
		// w.Header()["Location"] = []string{redirect}
		// w.WriteHeader(http.StatusSeeOther)
		// http.Redirect(w, r, redirect, http.StatusFound)
		// http.Redirect(w, r, redirect, http.StatusFound)
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		// http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, tokenString)
}

func logout(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Logout: delete auth cookie")
	responseDeleteAuthCookie(w)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

//
// auth middleware
//

// func authHandler(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 	})
// }

// with redirect to login page
func authHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Treat /login and /login/ prefix differently
		/*
			verifiedHeaderAuth := requestVerifyAuthHeader(r)
			verifiedCookieAuth := requestVerifyAuthCookie(r)
			if r.URL.Path == "/login" || strings.HasPrefix(r.URL.Path, "/login/") {
				// if requestVerifyAuthHeader(r) || requestVerifyAuthCookie(r) {
				if verifiedHeaderAuth || verifiedCookieAuth {
					http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
					return
				}
			} else {
				// if !requestVerifyAuthHeader(r) && !requestVerifyAuthCookie(r) {
				if !verifiedHeaderAuth && !verifiedCookieAuth {
					// w.WriteHeader(http.StatusUnauthorized)
					http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
					return
				}
			}
		*/
		claims := requestAuthClaims(r)
		// redirect to /login if not authenticated
		if claims == nil {
			if _, prs := r.Header["Authorization"]; prs {
				// Authorization header present, but invalid - most likely an API call, so don't redirect
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			redirect := strings.TrimPrefix(r.RequestURI, "/")
			if len(redirect) > 0 {
				redirect = "/login/?r=" + url.QueryEscape(redirect)
			} else {
				redirect = "/login/"
			}
			http.Redirect(w, r, redirect, http.StatusSeeOther)
			// http.Redirect(w, r, "/login/", http.StatusTemporaryRedirect)
			return
		}
		if shouldRefreshJWT(claims) {
			// log.Println("Updating token for", claims["email"])
			log.Println("Updating token for JWT with claims:", claims)
			tokenString := generateJWTWithClaims(claims)
			if requestVerifyAuthHeader(r) {
				w.Header()["Authorization"] = []string{"Bearer " + tokenString}
			}
			if requestVerifyAuthCookie(r) {
				responseSetAuthCookie(tokenString, w)
			}
		}
		// https://gocodecloud.com/blog/2016/11/15/simple-golang-http-request-context-example/
		// http://go-talks.appspot.com/github.com/dkondratovych/golang-ua-meetup/go-context/ctx.slide
		// create context with user id and JWT claims
		auth := AuthInfo{ID: claims["id"].(string), Admin: claims["admin"].(bool), Claims: claims}
		ctx := context.WithValue(r.Context(), "auth", auth)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
	// log response: https://medium.com/@matryer/the-http-handler-wrapper-technique-in-golang-updated-bc7fbcffa702
}

func loginAuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// redirect to root if already authenticated
		if claims := requestAuthClaims(r); claims != nil {
			log.Println("REFERER:", r.Referer())
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// if not authorized by token, return unauthorized
func headerAuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// if requestVerifyAuthHeader(r) {
		if claims := requestAuthHeaderClaims(r); claims != nil {
			if shouldRefreshJWT(claims) {
				// log.Println("Updating token for", claims["email"])
				log.Println("Updating token for JWT with claims:", claims)
				tokenString := generateJWTWithClaims(claims)
				w.Header()["Authorization"] = []string{"Bearer " + tokenString}
			}
			// https://gocodecloud.com/blog/2016/11/15/simple-golang-http-request-context-example/
			// http://go-talks.appspot.com/github.com/dkondratovych/golang-ua-meetup/go-context/ctx.slide
			// create context with user id and JWT claims
			auth := AuthInfo{ID: claims["id"].(string), Admin: claims["admin"].(bool), Claims: claims}
			ctx := context.WithValue(r.Context(), "auth", auth)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
}
