package main

import (
	// "crypto/md5"
	"encoding/json"
	"fmt"
	// "log"
	"net/http"
	"strings"
	"time"
	// jwt "github.com/dgrijalva/jwt-go"
	// "github.com/satori/go.uuid"
	// "io"
	// "io/ioutil"
	// "html"
	// "database/sql"
	// pq "github.com/lib/pq"
)

func apiGetNamedEntities(w http.ResponseWriter, r *http.Request) {

	// TODO: add auth

	db := getDB()

	// rows, err := db.Query("SELECT id, baseform, type FROM entities LIMIT $1", 3)
	rows, err := db.Query("SELECT id, baseform, type FROM entities")
	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()

	type Row struct {
		Id       string `json:"id"`
		Baseform string `json:"baseForm"`
		Typ      string `json:"type"`
		RelCount int    `json:"relationshipCount"`
	}

	data := []Row{}
	row := Row{}

	for rows.Next() {
		// var id sting
		// var baseform string
		// var typ string
		err = rows.Scan(&row.Id, &row.Baseform, &row.Typ)
		if err != nil {
			// handle this error
			fmt.Println("Error parsing row for namedEntities API call:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
			// panic(err)
		}
		// fmt.Println(id, baseform, typ)
		// fmt.Println(row)
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
	json.NewEncoder(w).Encode(struct {
		ServerTime int64 `json:"serverEpochTime"`
		Entities   []Row `json:"entities"`
	}{ServerTime: time.Now().Unix(), Entities: data})
	// fmt.Println(data)
}

func apiGetNamedEntity(w http.ResponseWriter, r *http.Request, id string) {

	// userID := r.Context().Value("auth").(AuthInfo).ID

	db := getDB()

	entity := struct {
		ID            string        `json:"id"`
		Type          string        `json:"type"`
		BaseForm      string        `json:"baseForm"`
		Datetime      time.Time     `json:"timeAdded"`
		Mentions      []interface{} `json:"mentions"`
		Relationships []interface{} `json:"relationships"`
	}{}

	entity.ID = id

	err := db.QueryRow(`
		SELECT type, baseform, datetime, relations
		FROM entities
		WHERE id = $1;
	`, id).Scan(&entity.Type, &entity.BaseForm, &entity.Datetime, &entity.Relationships)
	if err != nil {
		if err == ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println("db error: get entity error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rows, err := db.Query(`
		SELECT data FROM news_item_entities JOIN news_items ON news_item = news_items.id WHERE entity_id = $1 ORDER BY news_items.datetime DESC;
	`, id)
	if err != nil {
		fmt.Println("db error: entity mentions:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var mentions = make([]interface{}, 0, 100)

	for rows.Next() {
		var (
			data map[string]interface{}
		)
		// feedGroup := make(map[string]interface{})
		err = rows.Scan(&data)
		if err != nil {
			apiLogf(r, "error getting news item data from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mention := make(map[string]interface{})
		mention["id"] = data["id"]
		mention["contentDetectedLangCode"] = data["detectedLangCode"]
		mention["engTitle"] = data["title"].(map[string]interface{})["english"]
		mention["sourceItemOriginFeedName"] = data["source"].(map[string]interface{})["name"]
		mention["sourceItemType"] = data["mediaItemType"]
		mention["timeAdded"] = data["timeAdded"]
		mentions = append(mentions, mention)
	}

	entity.Mentions = mentions

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entity)
}

func apiNamedEntities(w http.ResponseWriter, r *http.Request) {

	get := r.Method == "GET"
	// post := r.Method == "POST"
	// patch := r.Method == "PATCH"
	// delete := r.Method == "DELETE"

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) > 0 && parts[0] == "" {
		parts = parts[1:]
	}

	p := 0
	nparts := len(parts)

	if nparts <= p || parts[p] != "namedEntities" {
		panic("invalid path prefix")
	}

	p++

	if nparts == p {

		if get {

			apiGetNamedEntities(w, r)
		}

	} else if nparts > p {

		entityID := parts[p]
		p++

		if nparts == p {

			if get {

				apiGetNamedEntity(w, r, entityID)
			}
		}

	}
}
