package main

import (
	"encoding/json"
	"fmt"
	// "log"
	// "github.com/jackc/pgx"
	// "github.com/jackc/pgx/pgtype"
	"github.com/satori/go.uuid"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type GeoLocationConstraint struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Radius    float64 `json:"radius"`
	ItemType  string  `json:"items"`
}

func apiGetUserQueries(w http.ResponseWriter, r *http.Request, userID string) {

	db := getDB()

	rows, err := db.Query(`
		--SELECT id, name, "user", feed_groups
		SELECT id, name, "user", data, entities, datetime,
			--feed_groups
			to_json(ARRAY(
				SELECT json_build_object('id',id,'name',name) FROM feed_groups
				WHERE id = ANY(SELECT * FROM json_array_elements_text(queries.feed_groups::json))
			))
		FROM queries WHERE "user" = $1;
	`, userID)
	if err != nil {
		apiLogf(r, "error getting feed groups from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	queries := make([]interface{}, 0)

	for rows.Next() {
		// var query struct {
		// 	ID   string `json:"id"`
		// 	Name string `json:"name"`
		// 	User string `json:"user"`
		// 	// FeedGroups   []string `json:"feedGroups"`
		// 	FeedGroups   interface{} `json:"feedGroups"`
		// 	Entities     []string    `json:"namedEntities"`
		// 	EntityFilter string      `json:"namedEntityFilterType"`
		// }
		// var data map[string]interface{}
		var (
			ID         string
			user       string
			data       map[string]interface{}
			feedGroups interface{}
			name       string   // for backwards compatibility
			entities   []string // for backwards compatibility
			datetime   time.Time
		)
		err = rows.Scan(&ID, &name, &user, &data, &entities, &datetime, &feedGroups)
		// err = rows.Scan(&query.ID, &query.Name, &query.User, &data, &query.Entities, &query.FeedGroups)
		// err = rows.Scan(&query.ID, &query.Name, &query.User, &query.FeedGroups, &query.Entities)
		if err != nil {
			apiLogf(r, "error getting row values: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// query.EntityFilter = "OR"
		data["id"] = ID
		if len(user) > 0 {
			data["user"] = user
		}
		// if len(name) > 0 {
		// 	data["name"] = name
		// }
		// if len(entities) > 0 {
		// 	data["namedEntities"] = entities
		// }
		// for backwards compatibility
		if _, prs := data["name"]; !prs {
			data["name"] = name
		}
		// for backwards compatibility
		if _, prs := data["entities"]; !prs {
			data["entities"] = entities
		}
		// if _, prs := data["namedEntities"]; !prs {
		// 	data["namedEntities"] = entities
		// }
		data["feedGroups"] = feedGroups
		// data["namedEntityFilterType"] = "OR"
		data["timeAdded"] = datetime

		queries = append(queries, data)
		// queries = append(queries, query)
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(queries)
}

func apiGetQuery(w http.ResponseWriter, r *http.Request, queryID string) {

	db := getDB()

	// var query struct {
	// 	ID           string      `json:"id"`
	// 	Name         string      `json:"name"`
	// 	User         string      `json:"user"`
	// 	Entities     []string    `json:"namedEntities"`
	// 	FeedGroups   interface{} `json:"feedGroups"`
	// 	EntityFilter string      `json:"namedEntityFilterType"`
	// }

	// err := db.QueryRow(`
	// 	SELECT id, name, "user", entities,
	// 		to_json(ARRAY(SELECT json_build_object('id',id,'name',name) FROM feed_groups
	// 		WHERE id = ANY(SELECT * FROM json_array_elements_text(queries.feed_groups::json))
	// 		))
	// 	FROM queries WHERE id = $1;
	// `, queryID).Scan(&query.ID, &query.Name, &query.User, &query.Entities, &query.FeedGroups)
	// if err != nil {
	// 	apiLogf(r, "error getting query from db: %v", err)
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }
	var (
		ID         string
		user       string
		data       map[string]interface{}
		feedGroups interface{}
		name       string   // for backwards compatibility
		entities   []string // for backwards compatibility
		datetime   time.Time
	)
	err := db.QueryRow(`
		SELECT id, "user", data, name, entities, datetime,
			to_json(ARRAY(
				SELECT json_build_object('id',id,'name',name) FROM feed_groups
				WHERE id = ANY(SELECT * FROM json_array_elements_text(queries.feed_groups::json))
			))
		FROM queries WHERE id = $1;
	`, queryID).Scan(&ID, &user, &data, &name, &entities, &datetime, &feedGroups)
	if err != nil {
		if err == ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		apiLogf(r, "error getting query from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data["id"] = ID
	if len(user) > 0 {
		data["user"] = user
	}
	// if len(name) > 0 {
	// 	data["name"] = name
	// }
	// if len(entities) > 0 {
	// 	data["namedEntities"] = entities
	// }
	// for backwards compatibility
	if _, prs := data["name"]; !prs {
		data["name"] = name
	}
	// if _, prs := data["namedEntities"]; !prs {
	// 	data["namedEntities"] = entities
	// }
	// for backwards compatibility
	if _, prs := data["entities"]; !prs {
		data["entities"] = entities
	}
	data["feedGroups"] = feedGroups
	// data["namedEntityFilterType"] = "OR"
	data["timeAdded"] = datetime

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

/*
func apiGetUserQuery(w http.ResponseWriter, r *http.Request, userID string, queryID string) {

	db := getDB()

	var query struct {
		ID           string      `json:"id"`
		Name         string      `json:"name"`
		User         string      `json:"user"`
		Entities     []string    `json:"namedEntities"`
		FeedGroups   interface{} `json:"feedGroups"`
		EntityFilter string      `json:"namedEntityFilterType"`
	}

	err := db.QueryRow(`
		SELECT id, name, "user", entities,
			to_json(ARRAY(SELECT json_build_object('id',id,'name',name) FROM feed_groups
			WHERE id = ANY(SELECT * FROM json_array_elements_text(queries.feed_groups::json))
			))
		FROM queries WHERE id = $1 AND "user" = $2;
	`, queryID, userID).Scan(&query.ID, &query.Name, &query.User, &query.Entities, &query.FeedGroups)
	if err != nil {
		apiLogf(r, "error getting query from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	query.EntityFilter = "OR"

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(query)
}
*/

// func apiPostUserQuery(w http.ResponseWriter, r *http.Request, userID string) {
func apiPostQuery(w http.ResponseWriter, r *http.Request) {

	// var query struct {
	// 	ID           string      `json:"id"`
	// 	Name         string      `json:"name"`
	// 	User         string      `json:"user"`
	// 	FeedGroups   interface{} `json:"feedGroups"`
	// 	Entities     []string    `json:"namedEntities"`
	// 	EntityFilter string      `json:"namedEntityFilterType"`
	// }
	query := make(map[string]interface{})

	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// query.EntityFilter = "OR" // hardcoded for now

	// id := uuid.NewV4().String()
	uid, err := uuid.NewV4()
	if err != nil {
		apiLogf(r, "error generating uuid: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	id := uid.String()
	// query.ID = id
	query["id"] = id

	// use authenticated user id if "user" is not present in the input query object
	user := r.Context().Value("auth").(AuthInfo).ID
	if _, prs := query["user"]; !prs {
		query["user"] = user
	}

	db := getDB()

	// _, err := db.Exec(`
	// 	INSERT INTO queries (id, name, "user", entities, feed_groups)
	// 	VALUES($1, $2, $3, $4, $5);
	// `, id, query.Name, query.User, query.Entities, query.FeedGroups)
	// if err != nil {
	// 	apiLogf(r, "error inserting into db: %v", err)
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }
	_, err = db.Exec(`
		INSERT INTO queries (id, "user", feed_groups, data)
		VALUES($1, $2, $3, $4);
	`, id, query["user"], query["feedGroups"], query)
	if err != nil {
		apiLogf(r, "error inserting into db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// apiGetQuery(w, r, id)

	// feedGroups := query.FeedGroups.([]interface{})
	feedGroups := query["feedGroups"].([]interface{})
	feedGroupIDs := make([]string, 0, len(feedGroups))
	for _, feedGroup := range feedGroups {
		feedGroupIDs = append(feedGroupIDs, feedGroup.(string))
	}

	rows, err := db.Query(`
		SELECT id, name
		FROM feed_groups
		WHERE id = ANY($1);
	`, feedGroupIDs)
	if err != nil {
		apiLogf(r, "error getting feed groups from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// feedGroups := make([]interface{}, 0, len(query.FeedGroups.([]interface{})))
	feedGroups = make([]interface{}, 0, len(feedGroups))

	for rows.Next() {
		var feedGroup struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		err = rows.Scan(&feedGroup.ID, &feedGroup.Name)
		if err != nil {
			apiLogf(r, "error getting row values: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		feedGroups = append(feedGroups, feedGroup.ID)
	}
	// query.FeedGroups = feedGroups
	query["feedGroups"] = feedGroups

	// if _, prs := query["namedEntityFilterType"]; !prs {
	// 	query["namedEntityFilterType"] = "OR" // hardcoded for now
	// }
	query["timeAdded"] = time.Now().UTC()

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(query)
}

func apiPatchQuery(w http.ResponseWriter, r *http.Request, id string) {

	// var query struct {
	// 	ID           string      `json:"id"`
	// 	Name         string      `json:"name"`
	// 	User         string      `json:"user"`
	// 	FeedGroups   interface{} `json:"feedGroups"`
	// 	Entities     []string    `json:"namedEntities"`
	// 	EntityFilter string      `json:"namedEntityFilterType"`
	// }
	query := make(map[string]interface{})

	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// query.EntityFilter = "OR" // hardcoded for now

	// if len(query.ID) > 0 && id != query.ID {
	// 	apiLogf(r, "path ID and payload ID mismatch: %v != %v", id, query.ID)
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	return
	// } else if len(query.ID) == 0 {
	// 	query.ID = id
	// }

	if query["id"] != nil && len(query["id"].(string)) > 0 && id != query["id"].(string) {
		apiLogf(r, "path ID and payload ID mismatch: %v != %v", id, query["id"])
		w.WriteHeader(http.StatusBadRequest)
		return
	} else if query["id"] == nil || len(query["id"].(string)) == 0 {
		query["id"] = id
	}

	// use authenticated user id if "user" is not present in the input query object
	// user := r.Context().Value("auth").(AuthInfo).ID
	// if _, prs := query["user"]; !prs {
	// 	query["user"] = user
	// }

	db := getDB()

	_, err := db.Exec(`
		--UPDATE queries SET datetime = timezone('utc'::text, now()), "user" = $2, feed_groups = $3, data = $4 WHERE id = $1;
		UPDATE queries SET datetime = timezone('utc'::text, now()), feed_groups = $2, data = $3 WHERE id = $1;
	`, id, query["feedGroups"], query) // do not set user when patching
	// `, id, query["user"], query["feedGroups"], query)
	if err != nil {
		apiLogf(r, "error inserting into db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// apiGetQuery(w, r, id)

	// feedGroups := query.FeedGroups.([]interface{})
	feedGroups := query["feedGroups"].([]interface{})
	feedGroupIDs := make([]string, 0, len(feedGroups))
	for _, feedGroup := range feedGroups {
		feedGroupIDs = append(feedGroupIDs, feedGroup.(string))
	}

	rows, err := db.Query(`
		SELECT id, name
		FROM feed_groups
		WHERE id = ANY($1);
	`, feedGroupIDs)
	if err != nil {
		apiLogf(r, "error getting feed groups from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// feedGroups := make([]interface{}, 0, len(query.FeedGroups.([]interface{})))
	feedGroups = make([]interface{}, 0, len(feedGroups))

	for rows.Next() {
		var feedGroup struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		err = rows.Scan(&feedGroup.ID, &feedGroup.Name)
		if err != nil {
			apiLogf(r, "error getting row values: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		feedGroups = append(feedGroups, feedGroup.ID)
	}
	// query.FeedGroups = feedGroups
	query["feedGroups"] = feedGroups

	// if _, prs := query["namedEntityFilterType"]; !prs {
	// 	query["namedEntityFilterType"] = "OR" // hardcoded for now
	// }
	query["timeAdded"] = time.Now().UTC()

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(query)
}

func apiGetTrending(w http.ResponseWriter, r *http.Request) {

	apiGetQueryTrending(w, r, "all")
}

func apiDeleteQuery(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	_, err := db.Exec(`
		DELETE FROM queries WHERE id = $1;
	`, id)
	if err != nil {
		apiLogf(r, "error removing query: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getMediaItemsCollectionData(db DB, from *time.Time, till *time.Time, feeds *[]string, entities *[]string, limit int, offset int) (Rows, error) {

	args := []interface{}{}
	conditions := []string{}
	conditionstr := ""

	if feeds != nil && len(*feeds) > 0 {
		conditions = append(conditions, "ni.feed = ANY($"+strconv.Itoa(len(args)+1)+")")
		args = append(args, *feeds)
	}

	join_sql := ""
	if entities != nil && len(*entities) > 0 {
		// conditions = append(conditions, "nie.entity_baseform = ANY($"+strconv.Itoa(len(args)+1)+")")
		join_sql = "JOIN news_item_entities nie ON ni.id = nie.news_item"
		conditions = append(conditions, "nie.entity_id = ANY($"+strconv.Itoa(len(args)+1)+")")
		args = append(args, *entities)
	}

	if from != nil {
		conditions = append(conditions, "ni.datetime >= $"+strconv.Itoa(len(args)+1))
		args = append(args, *from)
	}
	if till != nil {
		conditions = append(conditions, "ni.datetime <= $"+strconv.Itoa(len(args)+1))
		args = append(args, *till)
	}

	if len(conditions) > 0 {
		conditionstr = " WHERE " + strings.Join(conditions, " AND ")
	}

	conditionstr += " ORDER BY ni.datetime DESC "

	if limit > 0 {
		conditionstr += " LIMIT $" + strconv.Itoa(len(args)+1)
		args = append(args, limit)
	}
	if offset > 0 {
		conditionstr += " OFFSET $" + strconv.Itoa(len(args)+1)
		args = append(args, offset)
	}

	rows, err := db.Query(`
		-- SELECT ni.id, ni.datetime, nie.entity_baseform, ni.data
		SELECT ni.id, ni.data
		FROM news_items ni
		`+join_sql+`
		`+conditionstr+`;`, args...)

	return rows, err
}

// from=... | YYYYmmdd[T]HHMMSS vai epoch
// till=...
func apiGetQueryMediaItemCollection(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	var (
		name       string
		entities   []string
		feedGroups []string
		feeds      []string
	)

	var err error

	qs := r.URL.Query()

	offset := 0
	if values := qs["offset"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			offset = value
		}
	}

	limit := 10
	if values := qs["limit"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			limit = value
		}
	}

	// ?epochTimeSecs=1510531199&pastHourString=-5&namedEntity=United+States
	epochTimeSecs := 0
	if values := qs["epochTimeSecs"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			epochTimeSecs = value
		}
	} else if len(values) == 0 {
		apiLogf(r, "error: bad request: epochTimeSecs not defined")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// ?epochTimeSecs=1510531199&pastHourString=-5&namedEntity=United+States
	pastHour := 0
	if values := qs["pastHourString"]; len(values) > 0 {
		value, err := strconv.Atoi(strings.TrimPrefix(values[len(values)-1], "-"))
		if err == nil {
			pastHour = value
		}
	} else if len(values) == 0 {
		apiLogf(r, "error: bad request: pastHourString not defined")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if values := qs["namedEntity"]; len(values) > 0 {
		for _, value := range values {
			entities = append(entities, value)
		}
	}

	binCount := 0
	if values := qs["bins"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			binCount = value
		}
	}

	binSize := int64(0)
	if values := qs["binsize"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			binSize = int64(value)
		}
	}

	// TODO: extra default value logic
	if binCount == 0 {
		binCount = 24
	}
	if binSize == int64(0) {
		binSize = int64(3600)
	}

	if id == "all" {
		name = "All"
	} else {
		err := db.QueryRow(`
			SELECT name, entities, feed_groups
			FROM queries WHERE id = $1;
		`, id).Scan(&name, &entities, &feedGroups)
		if err != nil {
			if err == ErrNoRows {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			apiLogf(r, "error getting query from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if len(feedGroups) > 0 {
		err := db.QueryRow(`
			SELECT DISTINCT feed
			FROM feed_group_feeds WHERE feed_group = ANY($1);
		`, feedGroups).Scan(&feeds)
		if err != nil && err != ErrNoRows {
			apiLogf(r, "error getting feed group feeds from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// now := time.Date(2017, 11, 12, 23, 59, 59, 0, time.UTC)

	// var from *time.Time = nil
	// var till *time.Time = nil

	now := time.Unix(int64(epochTimeSecs), 0)
	till := now.Add(-time.Duration(pastHour) * time.Hour)
	from := till.Add(-time.Hour)

	// till = now.Add(-time.Duration(pastHour) * time.Duration(binSize) * time.Second)
	// from = till.Add(-time.Duration(binSize) * time.Second)

	from = from.UTC()
	till = till.UTC()

	fmt.Println("epoch:", epochTimeSecs)
	fmt.Println("from:", from)
	fmt.Println("till:", till)
	// layout := "2006-01-02 15:04:05 -0700 MST 2006"
	layout := "2006-01-02 15:04:05"
	fmt.Println("from:", from.UTC().Format(layout))
	fmt.Println("till:", till.UTC().Format(layout))

	// binEnd := now.Unix()
	// binStart := binEnd - binSize*int64(binCount)

	rows, err := getMediaItemsCollectionData(db, &from, &till, &feeds, &entities, limit, offset)
	if err != nil {
		apiLogf(r, "error getting media items from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	mediaItems := make([]interface{}, 0, 100)
	for rows.Next() {
		var (
			itemID string
			data   interface{}
		)
		err = rows.Scan(&itemID, &data)
		if err != nil {
			apiLogf(r, "error getting row values: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mediaItem := data.(map[string]interface{})
		out := map[string]interface{}{}
		out["id"] = itemID
		titles := mediaItem["title"].(map[string]interface{})
		title, prs := titles["english"]
		if !prs || title == nil {
			title = titles["original"]
		}
		out["title"] = title
		// out["title"] = mediaItem["title"].(map[string]interface{})["english"]
		out["detectedLangCode"] = mediaItem["detectedLangCode"]
		out["detectedTopics"] = mediaItem["detectedTopics"]
		out["mediaItemType"] = mediaItem["mediaItemType"]
		out["sentiment"] = mediaItem["sentiment"]
		out["timeAdded"] = mediaItem["timeAdded"]
		out["source"] = mediaItem["source"]
		// data.(map[string]interface{})["id"] = itemID
		mediaItems = append(mediaItems, out)
	}

	result := struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		// Now              int64                     `json:"epochTimeSecs"`
		MediaItems []interface{} `json:"mediaItems"`
	}{ID: id, Name: name /* Now: now.Unix(), */, MediaItems: mediaItems}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func fullTextSearchToTSQuery(text string) string {
	if len(text) == 0 {
		return text
	}
	// spaces are left as is, and multiple special symbols are joined together
	s := regexp.MustCompile("([()!&|]+|[^)!&| ]+)").FindAllString(text, -1)
	ss := make([]string, 0, len(s)*2)
	for i, e := range s {
		if i == 0 {
			ss = append(ss, e)
			continue
		}
		l := s[i-1][0]
		if l != '(' && l != ')' && l != '!' && l != '&' && l != '|' {
			l = e[0]
			if l != ')' && l != '&' && l != '|' {
				ss = append(ss, "&")
			}
		} else if l == ')' {
			l = e[0]
			if l != ')' && l != '&' && l != '|' {
				ss = append(ss, "&")
			}
		}
		ss = append(ss, e)
	}
	text = strings.Join(ss, " ")
	return text
}

// -------

func getMediaItems(db DB, from *time.Time, till *time.Time, feeds *[]string,
	entities *[]string, entityIds *[]string, mediaTypes *[]string, languages *[]string,
	fullTextSearch string, limit int, offset int, clusterID int, geoloc *GeoLocationConstraint) (Rows, error) {

	args := []interface{}{}
	conditions := []string{}
	conditionstr := ""
	joins := []string{}

	if feeds != nil {
		conditions = append(conditions, "ni.feed = ANY($"+strconv.Itoa(len(args)+1)+")")
		args = append(args, *feeds)
	}

	if (entityIds != nil && len(*entityIds) > 0) || (entities != nil && len(*entities) > 0) {

		entityConditions := []string{}
		if entityIds != nil && len(*entityIds) > 0 {
			entityConditions = append(entityConditions, "nie.entity_id = ANY($"+strconv.Itoa(len(args)+1)+")")
			args = append(args, *entityIds)
		}

		if entities != nil && len(*entities) > 0 {
			// entityConditions = append(entityConditions, "nie.entity_baseform = ANY($"+strconv.Itoa(len(args)+1)+")")
			entityConditions = append(entityConditions,
				"lower(nie.entity_baseform) = ANY(SELECT lower(unnest($"+strconv.Itoa(len(args)+1)+"::text[])))")
			args = append(args, *entities)
		}

		if len(entityConditions) == 1 {
			conditions = append(conditions, entityConditions[0])
		} else if len(entityConditions) > 1 {
			conditions = append(conditions, "("+strings.Join(entityConditions, " OR ")+")")
		}

		join_sql := `JOIN news_item_entities nie ON ni.id = nie.news_item`
		joins = append(joins, join_sql)
	}

	if mediaTypes != nil && len(*mediaTypes) > 0 {
		conditions = append(conditions, "ni.type = ANY($"+strconv.Itoa(len(args)+1)+")")
		args = append(args, *mediaTypes)
	}

	if languages != nil && len(*languages) > 0 {
		conditions = append(conditions, "ni.lang = ANY($"+strconv.Itoa(len(args)+1)+")")
		args = append(args, *languages)
	}

	if from != nil {
		conditions = append(conditions, "ni.datetime >= $"+strconv.Itoa(len(args)+1))
		args = append(args, *from)
	}
	if till != nil {
		conditions = append(conditions, "ni.datetime <= $"+strconv.Itoa(len(args)+1))
		args = append(args, *till)
	}

	if clusterID > -1 {
		conditions = append(conditions, "cluster_id = $"+strconv.Itoa(len(args)+1))
		args = append(args, clusterID)
	}

	prefix_sql := ""
	join2_sql := ""

	if geoloc != nil {
		narg := len(args) + 1
		prefix_sql = `
			WITH entities_geo AS (
				SELECT id, baseform, type FROM entities
				WHERE ST_DWithin(geo::geography,ST_SetSRID(ST_MakePoint($` +
			strconv.Itoa(narg) + ",$" + strconv.Itoa(narg+1) + "),4326)::geography,$" + strconv.Itoa(narg+2) + `)
			)
		`
		args = append(args, geoloc.Longitude)
		args = append(args, geoloc.Latitude)
		args = append(args, geoloc.Radius)

		// join2_sql = `JOIN news_item_entities nie ON ni.id = nie.news_item JOIN entities_geo e ON e.id = nie.entity_id`
		// join_sql := `JOIN news_item_entities nie ON ni.id = nie.news_item`
		// joins = append(joins, join_sql)
		// join_sql = `JOIN entities_geo e ON e.id = nie.entity_id`
		// joins = append(joins, join_sql)

		if geoloc.ItemType == "topic" {
			join2_sql = `
			--JOIN entities_geo e ON lower(e.baseform) = ANY(ni.topics)
			JOIN
			(
				SELECT e.id, e.baseform
				FROM (
					SELECT DISTINCT topic FROM news_items_filtered ni, unnest(ni.topics) topic
				) t
				JOIN entities_geo e ON lower(e.baseform) = lower(topic)
				--JOIN entities ON lower(entities.baseform) = lower(topic)
				--JOIN entities_geo e ON lower(e.baseform) = ANY(ni.topics)
				--WHERE geo IS NOT NULL AND ST_AsText(geo) != 'POINT EMPTY'
			) x ON lower(x.baseform) = ANY(ni.topics)
			--) x ON e.id = x.id
			-- option 2:
			-- JOIN
			-- (
			-- 	SELECT id, baseform
			-- 	FROM (
			-- 		SELECT DISTINCT topic FROM news_items_filtered ni, unnest(ni.topics) topic
			-- 	) t
			-- 	JOIN entities ON lower(entities.baseform) = lower(topic)
			-- 	WHERE geo IS NOT NULL AND ST_AsText(geo) != 'POINT EMPTY'
			-- ) x ON lower(x.baseform) = ANY(ni.topics)
			-- JOIN entities_geo e ON e.id = x.id
			`
		} else {
			join2_sql = `JOIN news_item_entities nie ON ni.id = nie.news_item JOIN entities_geo e ON e.id = nie.entity_id`
			join2_sql += `
				WHERE e.type = 'places'
			`
		}
	}

	if len(fullTextSearch) > 0 {
		join_sql := `
			JOIN (
			SELECT id,
				setweight(to_tsvector('english',coalesce(data->'title'->>'english','')), 'A') ||
				setweight(to_tsvector('english',coalesce(data->'teaser'->>'english',data->'transcript'->'english'->>'text','')), 'B') ||
				setweight(to_tsvector('english',coalesce(data->'mainText'->>'english','')), 'C') ||
				setweight(to_tsvector('english',coalesce(data->>'summary','')), 'D')
					tsv,
				to_tsquery('english',$` + strconv.Itoa(len(args)+1) + `) query
			FROM news_items) ftsearch
			ON ni.id = ftsearch.id`
		joins = append(joins, join_sql)
		args = append(args, fullTextSearch)
		conditions = append(conditions, "tsv @@ query")
	}

	joins_sql := ""
	if len(joins) > 0 {
		joins_sql = strings.Join(joins, " ")
	}
	if len(conditions) > 0 {
		conditionstr = " WHERE " + strings.Join(conditions, " AND ")
	}

	if len(prefix_sql) > 0 {
		prefix_sql += ",\n"
	} else {
		prefix_sql += "WITH "
	}

	prefix_sql += `
		news_items_filtered AS (
			SELECT ni.* FROM news_items ni
			` + joins_sql + `
			` + conditionstr + `
		)
	`

	limitstr := ""
	if limit > 0 {
		limitstr = " LIMIT $" + strconv.Itoa(len(args)+1)
		args = append(args, limit)
	}
	if offset > 0 {
		limitstr += " OFFSET $" + strconv.Itoa(len(args)+1)
		args = append(args, offset)
	}

	// https://stackoverflow.com/questions/156114/best-way-to-get-result-count-before-limit-was-applied/8242764#8242764
	sql := `
		` + prefix_sql + `
		SELECT ni.id, ni.datetime, ni.data, count(*) OVER() total_count
		FROM news_items_filtered ni
		` + join2_sql + `
		GROUP BY ni.id, ni.datetime, ni.data
		ORDER BY ni.datetime DESC` + limitstr + `;`
	// sql := `
	// 	` + prefix_sql + `
	// 	SELECT ni.id, ni.datetime, ni.data, count(*) OVER() total_count
	// 	FROM news_items ni
	// 	` + joins_sql + `
	// 	` + conditionstr + `
	// 	GROUP BY ni.id
	// 	ORDER BY ni.datetime DESC` + limitstr + `;`

	if config.Debug {
		fmt.Println(sql, args)
	}

	rows, err := db.Query(sql, args...)

	return rows, err
}

func getTrendingData(db DB, from *time.Time, till *time.Time, feeds *[]string,
	entities *[]string, entityIds *[]string, mediaTypes *[]string, languages *[]string,
	fullTextSearch string, limit int, group bool) (rows Rows, err error) {

	args := []interface{}{}
	conditions := []string{}
	conditionstr := ""
	entities_conditions := []string{}
	entities_conditionstr := ""
	joins := []string{}

	if feeds != nil {
		conditions = append(conditions, "ni.feed = ANY($"+strconv.Itoa(len(args)+1)+")")
		args = append(args, *feeds)
	}

	if (entityIds != nil && len(*entityIds) > 0) || (entities != nil && len(*entities) > 0) {

		entityConditions := []string{}
		if entityIds != nil && len(*entityIds) > 0 {
			entityConditions = append(entityConditions, "nie.entity_id = ANY($"+strconv.Itoa(len(args)+1)+")")
			args = append(args, *entityIds)
		}

		if entities != nil && len(*entities) > 0 {
			// entityConditions = append(entityConditions, "nie.entity_baseform = ANY($"+strconv.Itoa(len(args)+1)+")")
			entityConditions = append(entityConditions,
				"lower(nie.entity_baseform) = ANY(SELECT lower(unnest($"+strconv.Itoa(len(args)+1)+"::text[])))")
			args = append(args, *entities)
		}

		if len(entityConditions) == 1 {
			conditions = append(conditions, entityConditions[0])
		} else if len(entityConditions) > 1 {
			conditions = append(conditions, "("+strings.Join(entityConditions, " OR ")+")")
		}

		join_sql := `JOIN news_item_entities nie ON ni.id = nie.news_item`
		joins = append(joins, join_sql)
	} else if group {
		join_sql := `JOIN news_item_entities nie ON ni.id = nie.news_item`
		joins = append(joins, join_sql)
	}

	if mediaTypes != nil && len(*mediaTypes) > 0 {
		conditions = append(conditions, "ni.type = ANY($"+strconv.Itoa(len(args)+1)+")")
		args = append(args, *mediaTypes)
	}

	if languages != nil && len(*languages) > 0 {
		conditions = append(conditions, "ni.lang = ANY($"+strconv.Itoa(len(args)+1)+")")
		args = append(args, *languages)
	}

	if from != nil {
		conditions = append(conditions, "ni.datetime >= $"+strconv.Itoa(len(args)+1))
		args = append(args, *from)
	}
	if till != nil {
		conditions = append(conditions, "ni.datetime <= $"+strconv.Itoa(len(args)+1))
		args = append(args, *till)
	}

	if len(fullTextSearch) > 0 {
		join_sql := `
			JOIN (
			SELECT id,
				setweight(to_tsvector('english',coalesce(data->'title'->>'english','')), 'A') ||
				setweight(to_tsvector('english',coalesce(data->'teaser'->>'english',data->'transcript'->'english'->>'text','')), 'B') ||
				setweight(to_tsvector('english',coalesce(data->'mainText'->>'english','')), 'C') ||
				setweight(to_tsvector('english',coalesce(data->>'summary','')), 'D')
					tsv,
				to_tsquery('english',$` + strconv.Itoa(len(args)+1) + `) query
			FROM news_items) ftsearch
			ON ni.id = ftsearch.id`
		joins = append(joins, join_sql)
		args = append(args, fullTextSearch)
		conditions = append(conditions, "tsv @@ query")
	}

	joins_sql := ""
	if len(joins) > 0 {
		joins_sql = strings.Join(joins, " ")
	}
	if len(conditions) > 0 {
		conditionstr = " WHERE " + strings.Join(conditions, " AND ")
	}

	if group && ((entityIds != nil && len(*entityIds) > 0) || (entities != nil && len(*entities) > 0)) {

		entityConditions := []string{}
		if entityIds != nil && len(*entityIds) > 0 {
			entityConditions = append(entityConditions, "e.id = ANY($"+strconv.Itoa(len(args)+1)+")")
			args = append(args, *entityIds)
		}

		if entities != nil && len(*entities) > 0 {
			// entityConditions = append(entityConditions, "e.baseform = ANY($"+strconv.Itoa(len(args)+1)+")")
			entityConditions = append(entityConditions,
				"lower(e.baseform) = ANY(SELECT lower(unnest($"+strconv.Itoa(len(args)+1)+"::text[])))")
			args = append(args, *entities)
		}

		if len(entityConditions) == 1 {
			entities_conditions = append(entities_conditions, entityConditions[0])
		} else if len(entityConditions) > 1 {
			entities_conditions = append(entities_conditions, "("+strings.Join(entityConditions, " OR ")+")")
		}
	}

	if len(entities_conditions) > 0 {
		entities_conditionstr = " WHERE " + strings.Join(entities_conditions, " AND ")
	}

	limitstr := ""
	if limit > 0 && group {
		limitstr = " LIMIT $" + strconv.Itoa(len(args)+1)
		args = append(args, limit)
	}

	var sql string
	if group {
		if (entityIds != nil && len(*entityIds) > 0) || (entities != nil && len(*entities) > 0) {
			sql = `
			SELECT COALESCE(r.rate, 0), COALESCE(r.ids, ARRAY[]::text[]), COALESCE(r.dts, ARRAY[]::timestamp[]), e.baseform, e.id
			FROM entities e
			LEFT JOIN (
				SELECT count(ni.id) rate, array_agg(ni.id) ids, array_agg(ni.datetime) dts, nie.entity_baseform baseform, nie.entity_id id
				FROM news_items ni
				` + joins_sql + `
				` + conditionstr + `
				GROUP BY entity_baseform, entity_id
				ORDER BY rate DESC
			) r ON r.id = e.id ` +
				entities_conditionstr + `
			ORDER BY rate DESC, baseform` + limitstr + `;`
		} else {
			sql = `
			SELECT count(ni.id) rate, array_agg(ni.id), array_agg(ni.datetime), nie.entity_baseform, nie.entity_id
			FROM news_items ni
			` + joins_sql + `
			` + conditionstr + `
			GROUP BY entity_baseform, entity_id
			ORDER BY rate DESC, nie.entity_baseform` + limitstr + `;`
		}
	} else {
		// sql = `
		// 	SELECT count(ni.id) rate, array_agg(ni.id), array_agg(ni.datetime) --, nie.entity_baseform, nie.entity_id
		// 	FROM news_items ni
		// 	JOIN news_item_entities nie
		// 	ON ni.id = nie.news_item` +
		// 	conditionstr + `
		// 	--GROUP BY entity_baseform, entity_id
		// 	ORDER BY rate DESC` + limitstr + `;`
		sql = `
			SELECT count(*) rate, array_agg(id), array_agg(datetime) FROM (
			SELECT ni.id, ni.datetime
			FROM news_items ni
			` + joins_sql + `
			` + conditionstr + `
			GROUP BY ni.id
			ORDER BY ni.datetime DESC
			) t;`
	}

	if config.Debug {
		fmt.Println(sql, args)
	}

	rows, err = db.Query(sql, args...)

	return rows, err
}

func getTotalTrendingBins(db DB, feeds *[]string, entities []string, entityIds []string, mediaTypes []string, languages []string,
	from *time.Time, till *time.Time, fullTextSearch string, binCount int, binSize int64, limit int) (out map[string]int, err error) {

	rows, err := getTrendingData(db, from, till, feeds, &entities, &entityIds, &mediaTypes, &languages, fullTextSearch, limit, false)
	if err != nil {
		// apiLogf(r, "error getting trending statistics from db: %v", err)
		// w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// binCount := 24
	// binSize := int64(3600)
	// now := time.Date(2017, 11, 12, 23, 59, 59, 0, time.UTC)
	var end time.Time
	if till == nil {
		end = time.Now().UTC() // now
	} else {
		end = *till
	}
	binEnd := end.Unix()
	binStart := binEnd - binSize*int64(binCount)

	// [ { id: …, baseForm: …, bins: { <h>: <count> }, … next entity … ]
	// out = map[string]map[string]int{}
	// out = make([]map[string]interface{}, 0, 100)

	// bins := make([]int, binCount)
	for rows.Next() {
		var (
			count int
			// baseForm   string
			// entityID   string
			newsItems  []string
			timestamps []time.Time
			// newsItemID string
			// timestamp  time.Time
		)
		// SELECT ni.id, ni.datetime
		// err = rows.Scan(&count, &newsItems, &timestamps, &baseForm, &entityID)
		err = rows.Scan(&count, &newsItems, &timestamps)
		// err = rows.Scan(&newsItem, &timestamp)
		// fmt.Println(count, timestamps)
		// fmt.Println(count, newsItems, timestamps, baseForm)
		if err != nil {
			fmt.Println("error getting row values:", err)
			// apiLogf(r, "error getting row values: %v", err)
			// w.WriteHeader(http.StatusInternalServerError)
			return
		}
		bins := make([]int, binCount)
		for _, ts := range timestamps {
			tsux := ts.Unix()
			if tsux > binEnd || tsux < binStart {
				continue
			}
			tsux = tsux - binStart
			bins[int(tsux/binSize)] += 1
		}
		out = map[string]int{}
		for i, c := range bins {
			if c > 0 {
				out["-"+strconv.Itoa(binCount-i-1)] = c
			}
		}
	}
	return
}

func getTrendingBins(db DB, feeds *[]string, entities []string, entityIds []string, mediaTypes []string, languages []string,
	from *time.Time, till *time.Time, fullTextSearch string, binCount int, binSize int64, limit int) (out []map[string]interface{}, err error) {
	// from *time.Time, till *time.Time, binCount int, binSize int64, limit int) (out map[string]map[string]int, err error) {

	rows, err := getTrendingData(db, from, till, feeds, &entities, &entityIds, &mediaTypes, &languages, fullTextSearch, limit, true)
	if err != nil {
		// apiLogf(r, "error getting trending statistics from db: %v", err)
		// w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// binCount := 24
	// binSize := int64(3600)
	// now := time.Date(2017, 11, 12, 23, 59, 59, 0, time.UTC)
	var end time.Time
	if till == nil {
		end = time.Now().UTC() // now
	} else {
		end = *till
	}
	binEnd := end.Unix()
	binStart := binEnd - binSize*int64(binCount)

	// [ { id: …, baseForm: …, bins: { <h>: <count> }, … next entity … ]
	// out = map[string]map[string]int{}
	out = make([]map[string]interface{}, 0, 100)

	for rows.Next() {
		var (
			count      int
			baseForm   *string
			entityID   *string
			newsItems  []string
			timestamps []time.Time
		)
		err = rows.Scan(&count, &newsItems, &timestamps, &baseForm, &entityID)
		// fmt.Println(count, timestamps)
		// fmt.Println(count, newsItems, timestamps, baseForm)
		if err != nil {
			fmt.Println("error getting row values:", err)
			// apiLogf(r, "error getting row values: %v", err)
			// w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if baseForm == nil || entityID == nil {
			continue
			// fmt.Println(count, timestamps, baseForm, entityID)
		}
		// continue

		bins := make([]int, binCount)
		for _, ts := range timestamps {
			tsux := ts.Unix()
			if tsux > binEnd || tsux < binStart {
				continue
			}
			tsux = tsux - binStart
			bins[int(tsux/binSize)] += 1
		}
		binsOut := map[string]int{}
		for i, c := range bins {
			if c > 0 {
				binsOut["-"+strconv.Itoa(binCount-i-1)] = c
			}
		}
		out = append(out, map[string]interface{}{"id": entityID, "baseForm": baseForm, "bins": binsOut})
		// out[baseForm] = binsOut
		// fmt.Println(out)
	}
	return
}

func parseDatetime(input string, since *time.Time) *time.Time {

	if len(input) > 0 {
		if input[0] == '-' {
			if since == nil {
				now := time.Now()
				since = &now
			}
			// y - years
			// M|mo - months
			// w - weeks
			// d - days
			t, err := timeAddDurationString(since.UTC(), input)
			if err == nil {
				return &t
			}
		} else if input == "now" {
			// if since == nil {
			// 	now := time.Now()
			// 	since = &now
			// }
			// t := since.UTC()
			t := time.Now().UTC() // "now" always returns current UTC regardless of since value
			return &t
		} else if epoch, err := strconv.Atoi(input); err == nil {
			t := time.Unix(int64(epoch), 0).UTC()
			return &t
		} else if t, err := time.Parse(time.RFC3339, input); err == nil {
			return &t
		}
	} else {
		// if since == nil {
		// 	now := time.Now()
		// 	since = &now
		// }
		// t := since.UTC()
		// return &t
		// NOTE: returns nil on empty input
	}
	return nil
	// deprecated
	if len(input) > 0 {
		if m, err := regexp.MatchString("^\\d+$", input); err == nil && m {
			if epoch, err := strconv.Atoi(input); err == nil {
				t := time.Unix(int64(epoch), 0).UTC()
				return &t
			}
		} else if m, err := regexp.MatchString("^+\\d+h$", input); err == nil && m {
			if since == nil {
				now := time.Now().UTC()
				since = &now
			}
			// relative
			if pastHour, err := strconv.Atoi(input[1 : len(input)-1]); err == nil {
				t := since.UTC().Add(-time.Duration(pastHour) * time.Hour)
				return &t
			}
		} else if m, err := regexp.MatchString("^-\\d+h$", input); err == nil && m {
			if since == nil {
				now := time.Now().UTC()
				since = &now
			}
			// relative
			if pastHour, err := strconv.Atoi(input[1 : len(input)-1]); err == nil {
				t := since.UTC().Add(-time.Duration(pastHour) * time.Hour)
				return &t
			}

		} else {
			if t, err := time.Parse(time.RFC3339, input); err == nil {
				return &t
			}
		}
	}
	return nil
}

func apiGetQueryTrending(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	// var query struct {
	// 	ID           string      `json:"id"`
	// 	Name         string      `json:"name"`
	// 	User         string      `json:"user"`
	// 	Entities     []string    `json:"namedEntities"`
	// 	FeedGroups   interface{} `json:"feedGroups"`
	// 	EntityFilter string      `json:"namedEntityFilterType"`
	// }
	var (
		name       string
		entities   []string
		feedGroups []string
		feeds      *[]string
	)

	var err error

	qs := r.URL.Query()

	limit := 10
	if values := qs["limit"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			limit = value
		}
	}

	binCount := 0
	if values := qs["bins"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			binCount = value
		}
	}

	binSize := int64(0)
	if values := qs["binsize"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			binSize = int64(value)
		}
	}

	// TODO: extra default value logic
	if binCount == 0 {
		binCount = 24
	}
	if binSize == int64(0) {
		binSize = int64(3600)
	}

	if values := qs["feed"]; len(values) > 0 {
		_feeds := make([]string, len(values))
		feeds = &_feeds
		for i, value := range values {
			_feeds[i] = value
		}
	}

	if len(*feeds) == 0 {
		if id == "all" {
			name = "All"
		} else {
			err := db.QueryRow(`
				SELECT name, entities, feed_groups
				FROM queries WHERE id = $1;
			`, id).Scan(&name, &entities, &feedGroups)
			if err != nil {
				if err == ErrNoRows {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				apiLogf(r, "error getting query from db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}

	if len(feedGroups) > 0 {
		rows, err := db.Query(`
			SELECT DISTINCT feed
			FROM feed_group_feeds WHERE feed_group = ANY($1);
		`, feedGroups)
		if err != nil {
			apiLogf(r, "error getting feed group feeds from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var fds []string
		for rows.Next() {
			var feed string
			err := rows.Scan(&feed)
			if err != nil {
				apiLogf(r, "error getting feed group feed value from db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			fds = append(fds, feed)
		}
		feeds = &fds
	}

	// varianti kā parsēt laikus: timestamp, -NNNh, YYYYmmdd[T]HHMMSS[Z (UTC)

	// fmt.Println(r.URL.Query())

	now := time.Now().UTC()
	// now := time.Now()
	// now := time.Date(2017, 11, 12, 23, 59, 59, 0, time.UTC)
	// binEnd := now.Unix()
	// binCount := 24
	// binSize := int64(3600)
	// binStart := binEnd - binSize*int64(binCount)

	/*
		binLefts := make([]int64, binCount)
		for i := 0; i < binCount; i++ {
			binLefts[i] = binStart + int64(i)*binSize
		}

		queryEntities := map[string]bool{}
		queryEntityBins := map[string][]int{}
		for _, k := range entities {
			queryEntities[k] = true
			queryEntityBins[k] = make([]int, binCount)
		}
	*/

	// divi varianti: vispār nav feedi, tad nosacījumu atmetam - visi feedi
	// ir feedi, tad ir nosacījums - tikai izvēlētos feedos

	// rows, err := db.Query(`
	// 	SELECT count(ni.id) rate, array_agg(ni.id), array_agg(ni.datetime), nie.entity_baseform
	// 	FROM news_items ni
	// 	JOIN news_item_entities nie
	// 	ON ni.id = nie.news_item
	// 	AND nie.entity_baseform = ANY($1)
	// 	--AND ni.feed = ANY($1)
	// 	-- WHERE ni.datetime ...
	// 	-- AND ni.feed = ANY(... query feeds ...)
	// 	GROUP BY entity_baseform
	// 	ORDER BY rate DESC
	// 	-- LIMIT 10 -- top 10, bet tad vajag atsevišķu vaicājumu entītēm, kas ir kvērijā, jo tās varbūt atrodas stipri zem top10
	// 	LIMIT $2
	// 	;
	// `, entities, 10)
	// rows, err :=  getTrendingData(db DB, from *time.Time, till *time.Time, feeds *[]string, entities *[]string, limit *int) (Rows, error) {

	var from *time.Time = nil
	if values := qs["from"]; len(values) > 0 {
		from = parseDatetime(values[len(values)-1], nil)
		fmt.Println("FROM:", *from)
		// if value != nil {
		// 	from = value
		// }
	} else {
		from = parseDatetime("-24h", nil) // defaults to -24h
	}

	var till *time.Time = nil
	if values := qs["till"]; len(values) > 0 {
		till = parseDatetime(values[len(values)-1], nil)
		fmt.Println("TILL:", *till)
		// if value != nil {
		// 	till = value
		// }
	}

	var totalBins map[string]int
	// selectedEntityBins := map[string]map[string]int{}
	selectedEntityBins := make([]map[string]interface{}, 0, 100)
	if id != "all" && (len(entities) > 0 || (feeds != nil && len(*feeds) > 0)) {
		selectedEntityBins, err = getTrendingBins(db, feeds, entities, []string{}, []string{}, []string{}, from, till, "", binCount, binSize, limit)
		if err != nil {
			apiLogf(r, "[1] error getting trending statistics from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		totalBins, err = getTotalTrendingBins(db, feeds, entities, []string{}, []string{}, []string{}, from, till, "", binCount, binSize, limit)
	} else {
		totalBins, err = getTotalTrendingBins(db, feeds, []string{}, []string{}, []string{}, []string{}, from, till, "", binCount, binSize, limit)
	}
	if err != nil {
		apiLogf(r, "[2] error getting total trending statistics from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	entityBinsOut, err := getTrendingBins(db, feeds, []string{}, []string{}, []string{}, []string{}, from, till, "", binCount, binSize, limit)
	if err != nil {
		apiLogf(r, "[2] error getting trending statistics from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	/*
		rows, err := getTrendingData(db, nil, nil, &feeds, &entities, limit)
		if err != nil {
			apiLogf(r, "error getting trending statistics from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		entityBins := map[string][]int{}

		entityBinsOut := map[string]map[string]int{}

		for rows.Next() {
			// var feedGroup struct {
			// 	ID   string `json:"id"`
			// 	Name string `json:"name"`
			// }
			var (
				count      int
				baseForm   string
				newsItems  []string
				timestamps []time.Time
			)
			err = rows.Scan(&count, &newsItems, &timestamps, &baseForm)
			if err != nil {
				apiLogf(r, "error getting row values: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// _, queryEntity := queryEntities[baseForm]

			bins := make([]int, binCount)
			entityBins[baseForm] = bins
			// fmt.Println(newsItems)
			// fmt.Println(baseForm, timestamps, queryEntity)
			// feedGroups = append(feedGroups, feedGroup)
			for _, ts := range timestamps {
				tsux := ts.Unix()
				if tsux > binEnd {
					fmt.Printf(".")
					continue
				}
				tsux = tsux - binStart
				bins[int(tsux/binSize)] += 1
			}
			binsOut := map[string]int{}
			for i, c := range bins {
				if c > 0 {
					binsOut["-"+strconv.Itoa(binCount-i-1)] = c
				}
			}
			entityBinsOut[baseForm] = binsOut
			// fmt.Println("")
			// fmt.Println(bins, binsOut)
		}
		fmt.Println(entityBinsOut)
		// id: "0e883844-e482-4387-9901-75c714ad0875",
		// name: "London Fire",
		// epochTimeSecs: 1513224284,
		// selectedEntities: {
		// Fire: { },
		// London: {
		// -10: 1,
		// -13: 3,
		// -20: 1,
		// -4: 1
		// }
		// },
		// topKEntities: {
		// Brexit: {
		// -13: 3,
		// -4: 1
		// },
	*/

	result := struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Now  int64  `json:"epochTimeSecs"`
		// SelectedEntities map[string]map[string]int `json:"selectedEntities"`
		// TopKEntities     map[string]map[string]int `json:"topKEntities"`
		SelectedEntities []map[string]interface{} `json:"selectedEntities"`
		TopKEntities     []map[string]interface{} `json:"topKEntities"`
		TotalBins        map[string]int           `json:"totalBins"`
	}{ID: id, Name: name, Now: now.Unix(), TopKEntities: entityBinsOut, SelectedEntities: selectedEntityBins, TotalBins: totalBins}
	// }{ID: id, Name: name, Now: now.Unix(), TopKEntities: entityBinsOut, SelectedEntities: map[string]map[string]int{}}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func apiPostMediaItemsData(w http.ResponseWriter, r *http.Request) {

	var query struct {
		Feeds      []string      `json:"feeds"`
		FeedGroups []string      `json:"feedGroups"`
		Entities   []interface{} `json:"entities"`
		// EntityFilter string      `json:"namedEntityFilterType"`
		// From   *time.Time `json:"from"`
		// Till   *time.Time `json:"till"`
		From           string                 `json:"from"`
		Till           string                 `json:"till"`
		Offset         int                    `json:"offset"`
		Limit          int                    `json:"limit"`
		MediaTypes     []string               `json:"mediaTypes"`
		Languages      []string               `json:"languages"`
		FullTextSearch string                 `json:"fullTextSearch"`
		ClusterID      interface{}            `json:"cluster"`
		GeoLocation    *GeoLocationConstraint `json:"geoloc"`
	}

	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	db := getDB()

	var (
		entities  []string
		entityIds []string
		feeds     *[]string
		err       error
	)

	for _, entity := range query.Entities {
		switch value := entity.(type) {
		case string:
			entities = append(entities, value)
		case map[string]interface{}:
			if id, prs := value["id"]; prs {
				entityIds = append(entityIds, id.(string))
			}
		}
	}
	// fmt.Println(entities)
	// fmt.Println(entityIds)

	qs := r.URL.Query()

	/*
		limit := 10
		if values := qs["limit"]; len(values) > 0 {
			value, err := strconv.Atoi(values[len(values)-1])
			if err == nil {
				limit = value
			}
		}
	*/

	binCount := 0
	if values := qs["bins"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			binCount = value
		}
	}

	binSize := int64(0)
	if values := qs["binsize"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			binSize = int64(value)
		}
	}

	// TODO: extra default value logic
	if binCount == 0 {
		binCount = 24
	}
	if binSize == int64(0) {
		binSize = int64(3600)
	}

	if len(query.FeedGroups) > 0 {
		rows, err := db.Query(`
			SELECT DISTINCT feed
			FROM feed_group_feeds WHERE feed_group = ANY($1);
		`, query.FeedGroups)
		if err != nil {
			apiLogf(r, "error getting feed group feeds from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		// fds := []string{}
		for rows.Next() {
			var feed string
			err := rows.Scan(&feed)
			if err != nil {
				apiLogf(r, "error getting feed group feed value from db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// fds = append(fds, feed)
			query.Feeds = append(query.Feeds, feed)
		}
		// feeds = &fds
		feeds = &query.Feeds
	} else if len(query.Feeds) > 0 {
		feeds = &query.Feeds
	}

	// varianti kā parsēt laikus: timestamp, -NNNh, YYYYmmdd[T]HHMMSS[Z (UTC)

	// fmt.Println(r.URL.Query())

	now := time.Now().UTC()
	// now := time.Now()
	// now := time.Date(2017, 11, 12, 23, 59, 59, 0, time.UTC)
	// binEnd := now.Unix()
	// binCount := 24
	// binSize := int64(3600)
	// binStart := binEnd - binSize*int64(binCount)

	var from *time.Time = nil
	if values := qs["from"]; len(values) > 0 {
		from = parseDatetime(values[len(values)-1], nil)
		fmt.Println("FROM:", *from)
		// if value != nil {
		// 	from = value
		// }
	} else {
		from = parseDatetime("-24h", nil) // defaults to -24h
	}

	var till *time.Time = nil
	if values := qs["till"]; len(values) > 0 {
		till = parseDatetime(values[len(values)-1], nil)
		fmt.Println("TILL:", *till)
		// if value != nil {
		// 	till = value
		// }
		// now = *till
	}

	// if query.Till != nil {
	// 	now = *query.Till
	// }
	// if query.Till == nil {
	// 	*query.Till = now
	// }
	// if query.From == nil {
	// 	query.From = parseDatetime("-24h", query.Till)
	// }

	if query.Till == "" {
		till = &now
	} else {
		till = parseDatetime(query.Till, nil)
	}

	if query.From == "" {
		from = parseDatetime("-24", till)
	} else {
		from = parseDatetime(query.From, till)
	}

	// if query.Limit == 0 {
	// 	query.Limit = 100
	// }
	// fmt.Println("FROM:", query.From)
	// fmt.Println("TILL:", query.Till)
	// fmt.Println("FROM:", query.From.UTC())
	// fmt.Println("TILL:", query.Till.UTC())
	// fmt.Println("FROM:", query.From.UTC().Unix())
	// fmt.Println("TILL:", query.Till.UTC().Unix())
	// *query.Till = query.Till.UTC()

	var clusterID = -1
	var highlights interface{}
	if query.ClusterID != nil {
		clusterID = int(query.ClusterID.(float64))

		rows, err := db.Query(`
			SELECT highlights
			FROM clusters WHERE id = $1;
		`, clusterID)
		if err != nil {
			apiLogf(r, "error getting cluster data from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			err := rows.Scan(&highlights)
			if err != nil {
				apiLogf(r, "error getting cluster highlights data from db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}

	query.FullTextSearch = fullTextSearchToTSQuery(query.FullTextSearch)

	// rows, err := getMediaItems(db, query.From, query.Till, feeds, &entities, &entityIds, query.Limit, query.Offset)
	rows, err := getMediaItems(db, from, till, feeds, &entities, &entityIds, &query.MediaTypes, &query.Languages,
		query.FullTextSearch, query.Limit, query.Offset, clusterID, query.GeoLocation)
	if err != nil {
		apiLogf(r, "error getting media items from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var totalCount int64

	mediaItems := make([]interface{}, 0, 100)
	for rows.Next() {
		var (
			id       string
			datetime time.Time
			data     interface{}
		)
		// SELECT ni.id, ni.datetime, ni.data
		err := rows.Scan(&id, &datetime, &data, &totalCount)
		if err != nil {
			apiLogf(r, "error getting row values: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// fmt.Println(id)
		mediaItem := data.(map[string]interface{})
		mediaItem["id"] = id
		mediaItems = append(mediaItems, mediaItem)
	}

	result := struct {
		Now        int64         `json:"epochTimeSecs"`
		TotalCount int64         `json:"totalCount"`
		Limit      int           `json:"limit"`
		Offset     int           `json:"offset"`
		MediaItems []interface{} `json:"mediaItems"`
		Highlights interface{}   `json:"highlights"`
	}{Now: now.Unix(), TotalCount: totalCount, Limit: query.Limit, Offset: query.Offset, MediaItems: mediaItems, Highlights: highlights}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode(mediaItems)
	json.NewEncoder(w).Encode(result)
}

func apiPostTrendingData(w http.ResponseWriter, r *http.Request) {

	var query struct {
		Feeds      []string      `json:"feeds"`
		FeedGroups []string      `json:"feedGroups"`
		Entities   []interface{} `json:"entities"`
		// EntityFilter string      `json:"namedEntityFilterType"`
		// From *time.Time `json:"from"`
		// Till *time.Time `json:"till"`
		From           string   `json:"from"`
		Till           string   `json:"till"`
		MediaTypes     []string `json:"mediaTypes"`
		Languages      []string `json:"languages"`
		TotalOnly      bool     `json:"totalOnly"`
		FullTextSearch string   `json:"fullTextSearch"`
	}

	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	db := getDB()

	var (
		entities  []string
		entityIds []string
		feeds     *[]string
		err       error
	)

	for _, entity := range query.Entities {
		switch value := entity.(type) {
		case string:
			entities = append(entities, value)
		case map[string]interface{}:
			if id, prs := value["id"]; prs {
				entityIds = append(entityIds, id.(string))
			}
		}
	}
	// fmt.Println(entities)
	// fmt.Println(entityIds)

	qs := r.URL.Query()

	limit := 10
	if values := qs["limit"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			limit = value
		}
	}

	binCount := 0
	if values := qs["bins"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			binCount = value
		}
	}

	binSize := int64(0)
	if values := qs["binsize"]; len(values) > 0 {
		value, err := strconv.Atoi(values[len(values)-1])
		if err == nil {
			binSize = int64(value)
		}
	}

	// TODO: extra default value logic
	if binCount == 0 {
		binCount = 24
	}
	if binSize == int64(0) {
		binSize = int64(3600)
	}

	if len(query.FeedGroups) > 0 {
		rows, err := db.Query(`
			SELECT DISTINCT feed
			FROM feed_group_feeds WHERE feed_group = ANY($1);
		`, query.FeedGroups)
		if err != nil {
			apiLogf(r, "error getting feed group feeds from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		// var fds []string
		for rows.Next() {
			var feed string
			err := rows.Scan(&feed)
			if err != nil {
				apiLogf(r, "error getting feed group feed value from db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// fds = append(fds, feed)
			query.Feeds = append(query.Feeds, feed)
		}
		// feeds = &fds
		feeds = &query.Feeds
	} else if len(query.Feeds) > 0 {
		feeds = &query.Feeds
	}

	// varianti kā parsēt laikus: timestamp, -NNNh, YYYYmmdd[T]HHMMSS[Z (UTC)

	// fmt.Println(r.URL.Query())

	now := time.Now().UTC()
	// now := time.Now()
	// now := time.Date(2017, 11, 12, 23, 59, 59, 0, time.UTC)
	// binEnd := now.Unix()
	// binCount := 24
	// binSize := int64(3600)
	// binStart := binEnd - binSize*int64(binCount)

	var till *time.Time = nil
	if values := qs["till"]; len(values) > 0 {
		till = parseDatetime(values[len(values)-1], nil)
		fmt.Println("TILL:", *till)
		// if value != nil {
		// 	till = value
		// }
		// now = *till
	}

	// if query.Till != nil {
	// 	now = *query.Till
	// }

	var from *time.Time = nil
	if values := qs["from"]; len(values) > 0 {
		from = parseDatetime(values[len(values)-1], till)
		fmt.Println("FROM:", *from)
		// if value != nil {
		// 	from = value
		// }
	} else {
		from = parseDatetime("-24h", till) // defaults to -24h
	}

	if query.Till == "" {
		till = &now
	} else {
		till = parseDatetime(query.Till, nil)
	}

	if query.From == "" {
		from = parseDatetime("-24h", till)
	} else {
		from = parseDatetime(query.From, till)
	}

	fmt.Println("FROM:", from)
	fmt.Println("TILL:", till)

	query.FullTextSearch = fullTextSearchToTSQuery(query.FullTextSearch)

	var totalBins map[string]int
	// fmt.Println(query.FeedGroups)
	// fmt.Println(feeds)
	// fmt.Println(query.Entities)
	// selectedEntityBins := map[string]map[string]int{}
	selectedEntityBins := make([]map[string]interface{}, 0, 100)
	var entityBinsOut []map[string]interface{}
	if !query.TotalOnly {
		if len(query.Entities) > 0 || (feeds != nil && len(*feeds) > 0) {
			// selectedEntityBins, err = getTrendingBins(db, feeds, entities, entityIds, query.From, query.Till, binCount, binSize, limit)
			selectedEntityBins, err = getTrendingBins(db, feeds, entities, entityIds,
				query.MediaTypes, query.Languages, from, till, query.FullTextSearch, binCount, binSize, limit)
			if err != nil {
				apiLogf(r, "[3] error getting trending statistics from db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		// entityBinsOut, err := getTrendingBins(db, feeds, []string{}, []string{}, query.From, query.Till, binCount, binSize, limit)
		entityBinsOut, err = getTrendingBins(db, feeds, []string{}, []string{},
			query.MediaTypes, query.Languages, from, till, query.FullTextSearch, binCount, binSize, limit)
		if err != nil {
			apiLogf(r, "[4] error getting trending statistics from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// selectedEntityBins := map[string]map[string]int{}
	}

	if len(query.Entities) > 0 {
		totalBins, err = getTotalTrendingBins(db, feeds, entities, entityIds, query.MediaTypes, query.Languages,
			from, till, query.FullTextSearch, binCount, binSize, limit)
	} else {
		totalBins, err = getTotalTrendingBins(db, feeds, []string{}, []string{}, query.MediaTypes, query.Languages,
			from, till, query.FullTextSearch, binCount, binSize, limit)
	}
	if err != nil {
		apiLogf(r, "[5] error getting trending statistics from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	result := struct {
		ID               string                   `json:"id"`
		Name             string                   `json:"name"`
		Now              int64                    `json:"epochTimeSecs"`
		SelectedEntities []map[string]interface{} `json:"selectedEntities"`
		TopKEntities     []map[string]interface{} `json:"topKEntities"`
		// SelectedEntities map[string]map[string]int `json:"selectedEntities"`
		// TopKEntities     map[string]map[string]int `json:"topKEntities"`
		TotalBins map[string]int `json:"totalBins"`
	}{ID: "all", Name: "-", Now: till.Unix(), TopKEntities: entityBinsOut, SelectedEntities: selectedEntityBins, TotalBins: totalBins}
	// }{ID: "all", Name: "-", Now: now.Unix(), TopKEntities: entityBinsOut, SelectedEntities: selectedEntityBins}
	// }{ID: id, Name: name, Now: now.Unix(), TopKEntities: entityBinsOut, SelectedEntities: map[string]map[string]int{}}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func apiQueries(w http.ResponseWriter, r *http.Request) {

	// get := r.Method == "GET"
	post := r.Method == "POST"
	// patch := r.Method == "PATCH"
	// delete := r.Method == "DELETE"

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) > 0 && len(parts[0]) == 0 {
		parts = parts[1:]
	}

	if len(parts) == 0 || parts[0] != "queries" {
		panic("invalid path prefx") // TODO: fixme
	}

	parts = parts[1:]

	if len(parts) == 0 {

		if r.Method == "GET" {

			// apiGetQueries(w, r)

		} else if r.Method == "POST" {

			apiPostQuery(w, r)
		}

	} else if len(parts) > 0 {

		id := parts[0]
		parts = parts[1:]

		if len(parts) == 0 {

			if id == "trending" {

				if post {

					apiPostTrendingData(w, r)
				}

			} else if id == "mediaItems" {

				if post {

					apiPostMediaItemsData(w, r)
				}

			} else if r.Method == "GET" {

				apiGetQuery(w, r, id)

			} else if r.Method == "PATCH" {

				apiPatchQuery(w, r, id)

			} else if r.Method == "DELETE" {

				apiDeleteQuery(w, r, id)
			}
		} else if len(parts) > 0 {

			if parts[0] == "trending" && r.Method == "GET" {

				parts = parts[1:]

				if len(parts) > 0 {

					if parts[0] == "mediaItemSelection" {
						apiGetQueryMediaItemCollection(w, r, id)
					}

				} else {

					apiGetQueryTrending(w, r, id)
				}

				// if id == "all" {
				//
				// 	apiGetTrending(w, r)
				//
				// } else {
				// 	apiGetQueryTrending(w, r, id)
				// }
			}
		}
	}
}
