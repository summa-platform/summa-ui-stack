package main

import (
	"encoding/json"
	"fmt"
	// // "log"
	"net/http"
	"strings"
	"time"
	//
	// // jwt "github.com/dgrijalva/jwt-go"
	"github.com/satori/go.uuid"
)

func apiGetReportRatingTypes(w http.ResponseWriter, r *http.Request) {

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`[{"label":"Not Set","internalval":"not-set"},{"label":"Thumbs Up","internalval":"thumbs-up"},{"label":"Thumbs Down","internalval":"thumbs-down"}]`))
}

func apiGetReports(w http.ResponseWriter, r *http.Request) {

	// userID := r.Context().Value("auth").(AuthInfo).ID

	db := getDB()

	rows, err := db.Query(`
		SELECT id, datetime, data
		FROM reports
		ORDER BY datetime DESC;
	`)
	if err != nil {
		fmt.Println("db error: get reports error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}

	for rows.Next() {
		var (
			id       string
			datetime time.Time
			item     map[string]interface{}
		)
		err = rows.Scan(&id, &datetime, &item)
		if err != nil {
			fmt.Println("db error: scan reports result row error:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		item["id"] = id
		item["timeAdded"] = datetime
		items = append(items, item)
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
	json.NewEncoder(w).Encode(items)
}

func apiGetReport(w http.ResponseWriter, r *http.Request, id string) {

	// userID := r.Context().Value("auth").(AuthInfo).ID

	db := getDB()

	var (
		datetime time.Time
		item     map[string]interface{}
	)

	err := db.QueryRow(`
		SELECT datetime, data
		FROM reports
		WHERE id = $1;
	`, id).Scan(&datetime, &item)
	if err != nil {
		if err == ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println("db error: get report error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	item["id"] = id
	item["timeAdded"] = datetime

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(item)
}

func apiPostReport(w http.ResponseWriter, r *http.Request) {

	report := map[string]interface{}{}

	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// userID = r.Context().Value("auth").(AuthInfo).ID // retrieve id from auth token
	// id := uuid.NewV4().String()
	uid, err := uuid.NewV4()
	if err != nil {
		apiLogf(r, "error generating uuid: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	id := uid.String()
	datetime := time.Now().UTC()
	report["id"] = id
	report["timeAdded"] = datetime

	db := getDB()

	_, err = db.Exec(`
		INSERT INTO reports (id, datetime, data)
		VALUES($1, $2, $3)
	`, id, datetime, report)
	if err != nil {
		fmt.Println("db error: post report error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(report)
}

func apiPatchReport(w http.ResponseWriter, r *http.Request, id string) {

	report := map[string]interface{}{}

	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// userID = r.Context().Value("auth").(AuthInfo).ID // retrieve id from auth token
	// datetime := time.Now().UTC()
	// report["timeAdded"] = datetime
	report["id"] = id

	db := getDB()

	// _, err := db.Exec(`
	// 	UPDATE reports SET data = $2, datetime = $3 WHERE id = $1;
	// `, id, report, datetime)
	_, err := db.Exec(`
		UPDATE reports SET data = $2 WHERE id = $1;
	`, id, report)
	if err != nil {
		fmt.Println("db error: patch report error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(report)
}

func apiDeleteReport(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	_, err := db.Exec(`
		DELETE FROM reports WHERE id = $1;
	`, id)
	if err != nil {
		fmt.Println("db error: delete report error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func apiReports(w http.ResponseWriter, r *http.Request) {

	get := r.Method == "GET"
	post := r.Method == "POST"
	patch := r.Method == "PATCH"
	delete := r.Method == "DELETE"

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) > 0 && parts[0] == "" {
		parts = parts[1:]
	}

	p := 0
	nparts := len(parts)

	if nparts <= p || parts[p] != "feedback" {
		panic("invalid path prefix")
	}

	p++

	if nparts == p {

		if get {

			apiGetReports(w, r)

		} else if post {

			apiPostReport(w, r)
		}

	} else if nparts > p {

		reportID := parts[p]
		p++

		if nparts == p {

			if get && reportID == "ratingTypes" {

				apiGetReportRatingTypes(w, r)

			} else if get {

				apiGetReport(w, r, reportID)

			} else if patch {

				apiPatchReport(w, r, reportID)

			} else if delete {

				apiDeleteReport(w, r, reportID)
			}
		}

	}
	/*
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

							apiGetUserQuery(w, r, userID, queryID)

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
	*/
}
