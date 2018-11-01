package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	// "log"
	"net/http"
	"strings"
	"time"

	// jwt "github.com/dgrijalva/jwt-go"
	"github.com/satori/go.uuid"
)

func apiGetUsers(w http.ResponseWriter, r *http.Request) {

	db := getDB()

	authInfo := r.Context().Value("auth").(AuthInfo)

	var (
		rows Rows
		err  error
	)

	if !authInfo.Admin {
		rows, err = db.Query("SELECT id, name, email, role, suspended, data FROM users WHERE id = $1", authInfo.ID)
	} else {
		rows, err = db.Query("SELECT id, name, email, role, suspended, data FROM users")
	}

	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()

	type Row struct {
		Id        string                 `json:"id"`
		Name      string                 `json:"name"`
		Email     string                 `json:"email"`
		Role      string                 `json:"role"`
		Suspended bool                   `json:"isSuspended"`
		Data      map[string]interface{} `json:"data"`
	}

	data := []Row{}
	row := Row{}

	for rows.Next() {
		err = rows.Scan(&row.Id, &row.Name, &row.Email, &row.Role, &row.Suspended, &row.Data)
		if err != nil {
			// handle this error
			fmt.Println("Error parsing row for users API call:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
			// panic(err)
		}
		data = append(data, row)
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		fmt.Println("Error iterating rows for namedEntities API call:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
		// panic(err)
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode(data)
	json.NewEncoder(w).Encode(data)
	// fmt.Println(data)
}

func apiGetUser(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	authInfo := r.Context().Value("auth").(AuthInfo)

	if !authInfo.Admin && authInfo.ID != id {
		apiLogf(r, "only administrator can get other user's info")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	type Row struct {
		Id        string                 `json:"id"`
		Name      string                 `json:"name"`
		Email     string                 `json:"email"`
		Role      string                 `json:"role"`
		Suspended bool                   `json:"isSuspended"`
		Data      map[string]interface{} `json:"data"`
	}

	row := Row{}

	err := db.QueryRow("SELECT id, name, email, role, suspended, data FROM users WHERE id = $1", id).
		Scan(&row.Id, &row.Name, &row.Email, &row.Role, &row.Suspended, &row.Data)
	if err != nil {
		if err == ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println("db error: get user error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(row)
}

func apiPostUser(w http.ResponseWriter, r *http.Request) {

	authInfo := r.Context().Value("auth").(AuthInfo)

	if !authInfo.Admin {
		apiLogf(r, "only administrator can add new user")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var user struct {
		Id        string                 `json:"id"`
		Name      string                 `json:"name"`
		Email     string                 `json:"email"`
		Role      string                 `json:"role"`
		Password  string                 `json:"password"`
		Suspended bool                   `json:"isSuspended"`
		Data      map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		fmt.Println("error", err)
		w.WriteHeader(http.StatusBadRequest)
	} else {
	}

	if user.Id == "" {
		// user.Id = uuid.NewV4().String()
		if uid, err := uuid.NewV4(); err == nil {
			user.Id = uid.String()
		} else {
			fmt.Println("error generating uuid:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	db := getDB()

	_, err := db.Exec(`
	INSERT INTO users (id, name, email, role, suspended, password, data)
	VALUES($1, $2, $3, $4, $5, $6, $7)
	`, user.Id, user.Name, user.Email, user.Role, user.Suspended, fmt.Sprintf("%x", md5.Sum([]byte(user.Password))), user.Data)
	if err != nil {
		fmt.Println("Error inserting user into db:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)

}

func apiGetRoleTypes(w http.ResponseWriter, r *http.Request) {

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`[{"label":"User","internalval":"user"},{"label":"Administrator","internalval":"admin"}]`))
}

func verifyUserLogin(email string, password string) (bool, AuthClaims) {

	// hard-coded admin login
	// if email == "admin@summa" && password == "admin" {
	// 	return true, AuthClaims{"id": "admin-id"}
	// }

	db := getDB()

	var (
		id             string
		role           string
		storedPassword string
	)

	passwords := []string{fmt.Sprintf("%x", md5.Sum([]byte(password))), password}

	err := db.QueryRow(`
			SELECT id, role, password FROM users WHERE lower(email) = lower($1) AND password = ANY($2) AND suspended IS NOT TRUE;
		`, email, passwords).Scan(&id, &role, &storedPassword)
	if err != nil || len(id) == 0 {
		if len(config.ExternalUsersAPI) > 0 {
			var login struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			login.Email = email
			login.Password = password
			body, err := json.Marshal(login)
			params := strings.NewReader(string(body))
			r, err := http.Post(config.ExternalUsersAPI+"/checkPassword", "application/json", params)
			if err != nil {
				fmt.Println("error getting response from external user api:", err)
				return false, nil
			}
			if r.StatusCode != 200 {
				fmt.Println("response from external user api:", r.Status)
				return false, nil
			}
			user := make(map[string]interface{})
			if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
				fmt.Println("error parsing response from external user api:", err)
				return false, nil
			}
			id = user["id"].(string)
		} else {
			return false, nil
		}
	} else if storedPassword == password {
		// the password is stored in plaintext, convert it to hash
		_, err := db.Exec(`
			UPDATE users SET password = $2 WHERE id = $1;
		`, id, passwords[0])
		if err != nil {
			fmt.Println("DBError updating user password:", err)
			// w.WriteHeader(http.StatusInternalServerError)
			// return
		}
	}

	return true, AuthClaims{"id": id, "admin": role == "admin"}
}

func apiCheckUserPassword(w http.ResponseWriter, r *http.Request) {

	var user struct {
		Id        string                 `json:"id"`
		Name      string                 `json:"name"`
		Email     string                 `json:"email"`
		Role      string                 `json:"role"`
		Password  string                 `json:"password"`
		Suspended bool                   `json:"isSuspended"`
		Token     string                 `json:"token"`
		External  bool                   `json:"external"`
		Data      map[string]interface{} `json:"data"`
	}
	var credentials struct {
		User     string `json:"user"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	loginForm := r.Method == "POST" && r.Header["Content-Type"][0] == "application/x-www-form-urlencoded"

	if loginForm {
		r.ParseForm()
		credentials.User = r.PostFormValue("user")
		credentials.Email = r.PostFormValue("email")
		credentials.Password = r.PostFormValue("password")
	} else {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			fmt.Println("error", err)
			w.WriteHeader(http.StatusBadRequest)
		} else {
		}
	}

	// if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
	// 	fmt.Println("error", err)
	// 	w.WriteHeader(http.StatusBadRequest)
	// } else {
	// }

	// this function on success will return authorization to be used for all further api calls

	valid, claims := verifyUserLogin(credentials.Email, credentials.Password)
	// return ready to be used claims or better something other and have a fn to convert to claims ?
	// will we use some internal user struct ? probably no need, as only the id should be used for most of ops

	if !valid {
		// w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	tokenString := generateJWTWithClaims(claims)

	// user.Token = tokenString

	var id = claims["id"].(string)

	db := getDB()

	// err := db.QueryRow(`
	// 		SELECT id, name, email, role, suspended FROM users WHERE email = $1 AND password = $2 AND suspended IS NOT TRUE;
	// 	`, credentials.Email, credentials.Password).Scan(&user.Id, &user.Name, &user.Email, &user.Role, &user.Suspended)
	err := db.QueryRow(`
			SELECT id, name, email, role, suspended FROM users WHERE id = $1;
		`, id).Scan(&user.Id, &user.Name, &user.Email, &user.Role, &user.Suspended)
	if err != nil {
		if len(config.ExternalUsersAPI) > 0 {
			r, err := http.Get(config.ExternalUsersAPI + "/" + id)
			if err != nil {
				fmt.Println("error getting response from external user api:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if r.StatusCode != 200 {
				fmt.Println("response from external user api:", r.Status)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			user.External = true
			// user := make(map[string]interface{})
			if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
				fmt.Println("error parsing response from external user api:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			fmt.Println("Error getting user from db:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// w.Header()["Authorization"] = []string{"Bearer " + tokenString}
	responseSetAuthHeader(tokenString, w)

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(user)
}

func apiUpdateUser(w http.ResponseWriter, r *http.Request, id string) {

	type Base struct {
		ID        string                 `json:"id"`
		Name      string                 `json:"name"`
		Email     string                 `json:"email"`
		Role      string                 `json:"role"`
		Suspended bool                   `json:"isSuspended"`
		Data      map[string]interface{} `json:"data"`
	}

	var user struct {
		Base            `json:""`
		CurrentPassword string `json:"currentPassword"`
		Password        string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	db := getDB()

	user.Base.ID = id

	authInfo := r.Context().Value("auth").(AuthInfo)
	currentUserID := authInfo.ID

	if len(user.Password) > 0 {
		user.Password = fmt.Sprintf("%x", md5.Sum([]byte(user.Password)))
		// check password if the change is for current user, do not allow password change for other than current user if not administrator
		if !authInfo.Admin && currentUserID != id {
			apiLogf(r, "only administrator can change other user's password")
			w.WriteHeader(http.StatusBadRequest)
			return
		} else if currentUserID == id && len(user.CurrentPassword) > 0 {
			var password string
			err := db.QueryRow(`
					SELECT password FROM users WHERE id = $1;
				`, currentUserID).Scan(&password)
			if err != nil {
				if err == ErrNoRows {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				apiLogf(r, "db error: get user: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if password != user.CurrentPassword {
				apiLogf(r, "wrong password")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		} else if currentUserID == id {
			apiLogf(r, "current password not specified")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if authInfo.Admin {
			_, err := db.Exec(`
				UPDATE users SET name = $2, email = $3, role = $4, suspended = $5, password = $6, data = $7 WHERE id = $1;
			`, user.Base.ID, user.Base.Name, user.Base.Email, user.Base.Role, user.Base.Suspended, user.Password, user.Base.Data)
			if err != nil {
				fmt.Println("Error updating user into db:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			_, err := db.Exec(`
				UPDATE users SET name = $2, email = $3, password = $4, data = $5 WHERE id = $1;
			`, user.Base.ID, user.Base.Name, user.Base.Email, user.Password, user.Base.Data)
			if err != nil {
				fmt.Println("Error updating user into db:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	} else if authInfo.Admin {
		_, err := db.Exec(`
			UPDATE users SET name = $2, email = $3, role = $4, suspended = $5, data = $6 WHERE id = $1;
		`, user.Base.ID, user.Base.Name, user.Base.Email, user.Base.Role, user.Base.Suspended, user.Base.Data)
		if err != nil {
			fmt.Println("Error updating user into db:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		_, err := db.Exec(`
			UPDATE users SET name = $2, email = $3, data = $4 WHERE id = $1;
		`, user.Base.ID, user.Base.Name, user.Base.Email, user.Base.Data)
		if err != nil {
			fmt.Println("Error updating user into db:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user.Base)
}

func apiDeleteUser(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	_, err := db.Exec(`
		DELETE FROM users WHERE id = $1;
	`, id)
	if err != nil {
		fmt.Println("Error deleting user into db:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func apiGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("exp: %v claims: %v\n", r.Context().Value("auth").(AuthInfo).Claims["exp"].(float64), r.Context().Value("auth").(AuthInfo).Claims)
	// fmt.Printf("claims: %v\n", r.Context().Value("auth").(AuthInfo).Claims)

	var user struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
		// Password  string `json:"password"`
		Suspended bool `json:"isSuspended"`
		// Token     string `json:"token"`
		External bool                   `json:"external"`
		Data     map[string]interface{} `json:"data"`
	}

	id := r.Context().Value("auth").(AuthInfo).ID

	db := getDB()

	// err := db.QueryRow(`
	// 		SELECT id, name, email, role, suspended FROM users WHERE email = $1 AND password = $2 AND suspended IS NOT TRUE;
	// 	`, credentials.Email, credentials.Password).Scan(&user.Id, &user.Name, &user.Email, &user.Role, &user.Suspended)
	err := db.QueryRow(`
			SELECT id, name, email, role, suspended, data FROM users WHERE id = $1;
		`, id).Scan(&user.Id, &user.Name, &user.Email, &user.Role, &user.Suspended, &user.Data)
	if err != nil {
		// possibly no record in database, if external user api configured, try that
		if len(config.ExternalUsersAPI) > 0 {
			r, err := http.Get(config.ExternalUsersAPI + "/" + id)
			if err != nil {
				fmt.Println("error getting response from external user api:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if r.StatusCode != 200 {
				fmt.Println("response from external user api:", r.Status)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			user.External = true
			// user := make(map[string]interface{})
			if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
				fmt.Println("error parsing response from external user api:", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			fmt.Println("Error getting user from db:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// w.Header()["Authorization"] = []string{"Bearer " + tokenString}
	// responseSetAuthHeader(tokenString, w)

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(user)
}

func apiGetUserBookmarks(w http.ResponseWriter, r *http.Request, userID string) {

	userID = r.Context().Value("auth").(AuthInfo).ID

	db := getDB()

	rows, err := db.Query(`
		SELECT id, title, type, "user", datetime, item
		FROM bookmarks
		WHERE "user" = $1
		ORDER BY datetime DESC;
	`, userID)
	if err != nil {
		fmt.Println("db error: get bookmarks error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Row struct {
		ID       string                 `json:"id"`
		Title    string                 `json:"title"`
		Datetime time.Time              `json:"timeAdded"`
		Type     string                 `json:"type"`
		UserID   string                 `json:"userId"`
		Item     map[string]interface{} `json:"item"`
	}

	data := []Row{}

	for rows.Next() {
		row := Row{}
		err = rows.Scan(&row.ID, &row.Title, &row.Type, &row.UserID, &row.Datetime, &row.Item)
		if err != nil {
			fmt.Println("db error: scan bookmarks result row error:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		data = append(data, row)
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		fmt.Println("db error: row iteration error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func apiGetUserBookmark(w http.ResponseWriter, r *http.Request, userID string) {

	var row struct {
		ID       string                 `json:"id"`
		Title    string                 `json:"title"`
		Datetime time.Time              `json:"timeAdded"`
		Type     string                 `json:"type"`
		UserID   string                 `json:"userId"`
		Item     map[string]interface{} `json:"item"`
	}

	row.UserID = userID
	row.UserID = r.Context().Value("auth").(AuthInfo).ID // retrieve id from auth token

	db := getDB()

	err := db.QueryRow(`
		SELECT id, title, type, user, datetime, item
		FROM bookmarks
		WHERE id = $1;
	`, row.UserID).Scan(&row.ID, &row.Title, &row.Type, &row.UserID, &row.Datetime, &row.Item)
	if err != nil {
		if err == ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println("db error: get bookmarks error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(row)
}

func apiPostUserBookmarks(w http.ResponseWriter, r *http.Request, userID string) {

	var bookmark struct {
		ID       string                 `json:"id"`
		Title    string                 `json:"title"`
		Datetime time.Time              `json:"timeAdded"`
		Type     string                 `json:"type"`
		UserID   string                 `json:"userId"`
		Item     map[string]interface{} `json:"item"`
	}

	if err := json.NewDecoder(r.Body).Decode(&bookmark); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// bookmark.ID = uuid.NewV4().String()
	if uid, err := uuid.NewV4(); err == nil {
		bookmark.ID = uid.String()
	} else {
		apiLogf(r, "error generating uuid: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	bookmark.UserID = userID
	bookmark.UserID = r.Context().Value("auth").(AuthInfo).ID // retrieve id from auth token
	bookmark.Datetime = time.Now().UTC()

	db := getDB()

	_, err := db.Exec(`
		INSERT INTO bookmarks (id, title, type, "user", datetime, item)
		VALUES($1, $2, $3, $4, $5, $6)
	`, bookmark.ID, bookmark.Title, bookmark.Type, bookmark.UserID, bookmark.Datetime, bookmark.Item)
	if err != nil {
		fmt.Println("db error: post bookmark error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bookmark)
}

func apiPatchUserBookmark(w http.ResponseWriter, r *http.Request, bookmarkID string) {

	var bookmark struct {
		ID       string                 `json:"id"`
		Title    string                 `json:"title"`
		Datetime time.Time              `json:"timeAdded"`
		Type     string                 `json:"type"`
		UserID   string                 `json:"userId"`
		Item     map[string]interface{} `json:"item"`
	}

	if err := json.NewDecoder(r.Body).Decode(&bookmark); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	bookmark.UserID = r.Context().Value("auth").(AuthInfo).ID // retrieve id from auth token
	// bookmark.Datetime = time.Now().UTC()

	db := getDB()

	_, err := db.Exec(`
		UPDATE bookmarks SET title = $2, type = $3, "user" = $4, datetime = $5, item = $6 WHERE id = $1;
	`, bookmark.ID, bookmark.Title, bookmark.Type, bookmark.UserID, bookmark.Datetime, bookmark.Item)
	if err != nil {
		fmt.Println("db error: patch bookmark error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bookmark)
}

func apiDeleteUserBookmark(w http.ResponseWriter, r *http.Request, bookmarkID string) {

	userID := r.Context().Value("auth").(AuthInfo).ID // retrieve id from auth token

	db := getDB()

	_, err := db.Exec(`
		DELETE FROM bookmarks WHERE id = $1 AND "user" = $2;
	`, bookmarkID, userID)
	if err != nil {
		fmt.Println("db error: delete bookmark error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

/*

GET /users
POST /users
GET /users/roleTypes
POST /users/checkPassword
PATCH /users/<id>
DELETE /users/<id>

TODO: GET /users/current - get user specified by auth token

*/

func apiUsers(w http.ResponseWriter, r *http.Request) {

	get := r.Method == "GET"
	post := r.Method == "POST"
	patch := r.Method == "PATCH"
	delete := r.Method == "DELETE"

	parts := strings.Split(r.URL.Path, "/")

	if parts[0] == "" {
		parts = parts[1:]
	}

	p := 0
	nparts := len(parts)

	if nparts <= p || parts[p] != "users" {
		panic("invalid path prefix")
	}

	p++

	if nparts == p {

		if get {

			apiGetUsers(w, r)

		} else if post {

			apiPostUser(w, r)
		}

	} else if nparts > p {

		userID := parts[p]
		p++

		if nparts == p {

			if get && userID == "roleTypes" {

				apiGetRoleTypes(w, r)

			} else if get && userID == "current" {

				apiGetCurrentUser(w, r)

			} else if post && userID == "checkPassword" {

				apiCheckUserPassword(w, r)

			} else if get {

				apiGetUser(w, r, userID)

			} else if patch {

				apiUpdateUser(w, r, userID)

			} else if delete {

				apiDeleteUser(w, r, userID)
			}

		} else if nparts > p {

			if parts[p] == "queries" {

				p++

				if nparts == p {

					if get {

						apiGetUserQueries(w, r, userID)

					} else if post {

						// apiPostUserQuery(w, r, id)
					}
				} else if nparts > p {

					queryID := parts[p]
					p++

					if nparts == p && get {

						// apiGetUserQuery(w, r, userID, queryID)
						apiGetQuery(w, r, queryID)

					} else if nparts > p {

						if parts[p] == "trending" && get {

							if queryID == "all" {

								apiGetTrending(w, r)

							} else {

								apiGetQueryTrending(w, r, queryID)
							}
						}
					}
				}

			} else if parts[p] == "bookmarks" {

				p++

				if nparts == p {

					if get {

						apiGetUserBookmarks(w, r, userID)

					} else if post {

						apiPostUserBookmarks(w, r, userID)
					}

				} else if nparts > p {

					bookmarkID := parts[p]
					p++

					if nparts == p {

						if get {

							apiGetUserBookmark(w, r, bookmarkID)

						} else if patch {

							apiPatchUserBookmark(w, r, bookmarkID)

						} else if delete {

							apiDeleteUserBookmark(w, r, bookmarkID)
						}
					}
				}
			}
		}
	}
}
