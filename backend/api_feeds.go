package main

import (
	"encoding/json"
	"fmt"
	// "log"
	"net/http"
	"strings"
	"time"

	// pq "github.com/lib/pq"
	"github.com/jackc/pgx/pgtype"
	"github.com/satori/go.uuid"
)

func apiGetFeeds(w http.ResponseWriter, r *http.Request) {

	db := getDB()

	// rows, err := db.Query("SELECT data FROM feeds")
	// array_agg(array[feed_group_feeds.feed_group, feed_groups.name]) feed_groups
	rows, err := db.Query(`
		SELECT feeds.id,feeds.data,
		array_remove(array_agg(feed_group_feeds.feed_group),NULL) feed_group_ids,
		array_remove(array_agg(feed_groups.name),NULL) feed_group_names
		FROM feeds LEFT JOIN feed_group_feeds ON feeds.id = feed_group_feeds.feed
		LEFT JOIN feed_groups ON feed_groups.id = feed_group_feeds.feed_group
		GROUP BY feeds.id;
	`)
	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()

	var data []map[string]interface{}
	// row := Row{}
	// row := {}
	// var row string
	var feedString string
	// var feedGroups []interface{}
	var feedGroupIds []string
	var feedGroupNames []string
	var feedId string

	for rows.Next() {
		err = rows.Scan(&feedId, &feedString, &feedGroupIds, &feedGroupNames)
		if err != nil {
			// handle this error
			fmt.Println("Error parsing row for feeds API call:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
			// panic(err)
		}
		// fmt.Println(feedGroupIds)
		// fmt.Println(feedGroupNames)
		feedGroups := make([]map[string]string, len(feedGroupNames))
		for i := 0; i < len(feedGroupNames); i++ {
			feedGroups[i] = map[string]string{"id": feedGroupIds[i], "name": feedGroupNames[i]}
		}
		var feed map[string]interface{}
		json.Unmarshal([]byte(feedString), &feed)
		// feed["feedGroups"] = make([]interface{}, 0) // reset feedGroups
		feed["feedGroups"] = feedGroups
		feed["id"] = feedId // use id from id column
		data = append(data, feed)
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		fmt.Println("Error iterating rows for feeds API call:", err)
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

func apiPostFeed(w http.ResponseWriter, r *http.Request) {

	db := getDB()

	// var feed struct {
	// 	Id        string `json:"id"`
	// 	// Name      string `json:"name"`
	// 	// Email     string `json:"email"`
	// 	// Role      string `json:"role"`
	// 	// Password  string `json:"password"`
	// 	// Suspended bool   `json:"isSuspended"`
	// }

	var feed map[string]interface{}

	if err := json.NewDecoder(r.Body).Decode(&feed); err != nil {
		fmt.Println("error", err)
		w.WriteHeader(http.StatusBadRequest)
	} else {
	}

	fmt.Println(feed)

	if feed["id"] == "" || feed["id"] == nil {
		if uid, err := uuid.NewV4(); err == nil {
			feed["id"] = uid.String()
		} else {
			fmt.Println("error generating uuid:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Println("new feed id:", feed["id"])
	}

	id := feed["id"]
	// var feedString string
	feedData, err := json.Marshal(feed)

	_, err = db.Exec(`
	INSERT INTO feeds (id, data)
	VALUES($1, $2)
	`, id, feedData)
	if err != nil {
		fmt.Println("Error inserting user into db:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode(user)
	w.Write(feedData)

}

func apiGetFeedTypes(w http.ResponseWriter, r *http.Request) {

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`[{"label":"DW Endpoint","internalval":"dwFeed"},{"label":"BBC Endpoint","internalval":"bbcFeed"},{"label":"RSS Endpoint","internalval":"rssFeed"},{"label":"Twitter Endpoint","internalval":"twitterFeed"},{"label":"LV Endpoint","internalval":"lvFeed"}]`))
}

func apiUpdateFeed(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	var feedUpdate map[string]interface{}

	if err := json.NewDecoder(r.Body).Decode(&feedUpdate); err != nil {
		fmt.Println("error", err)
		w.WriteHeader(http.StatusBadRequest)
	} else {
	}

	delete(feedUpdate, "id")

	var storedData string

	err := db.QueryRow(`
		SELECT data FROM feeds WHERE id = $1;
	`, id).Scan(&storedData)
	if err != nil {
		if err == ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println("Error retrieving feed from db:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var feed map[string]interface{}

	err = json.Unmarshal([]byte(storedData), &feed)
	if err != nil {
		fmt.Println("Error retrieving feed from db:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	delete(feed, "feedGroups")

	for k, v := range feedUpdate {
		if k == "feedGroups" {
			// deal with this below
		} else {
			feed[k] = v
		}
	}

	// fmt.Println(feed)

	// var feedString string
	feedData, err := json.Marshal(feed)
	feed["id"] = id

	_, err = db.Exec(`
		UPDATE feeds SET data = $2 WHERE id = $1;
	`, id, feedData)
	if err != nil {
		fmt.Println("Error updating feed:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if feedGroups, ok := feedUpdate["feedGroups"]; ok {
		// TODO: update feed_groups table

		// https://stackoverflow.com/questions/25289593/bulk-upserts-within-a-sql-transaction-in-golang

		tx, err := db.Begin()
		if err != nil {
			fmt.Println("begin failed:", err)
		}

		stmt, err := tx.Prepare("update_feed_group_feeds", `
			INSERT INTO feed_group_feeds (feed, feed_group) VALUES ($1, $2)
			ON CONFLICT DO NOTHING;
		`)
		// ON DUPLICATE UPDATE balance SET money=money+? WHERE id=?
		if err != nil {
			fmt.Println("prepare statement failed:", err)
		}
		for _, gid := range feedGroups.([]interface{}) {
			_, err := tx.Exec(stmt.Name, id, gid.(string))
			if err != nil {
				fmt.Println("statement exec:", err)
			}
		}
		// err = stmt.Close()
		// if err != nil {
		// 	fmt.Println("statement close failed:", err)
		// }
		err = tx.Commit()
		if err != nil {
			fmt.Println("commit failed:", err)
		}

		// txn, err := db.Begin()
		// if err != nil {
		// 	log.Fatal(err)
		// }
		//
		// stmt, err := txn.Prepare(pq.CopyIn("users", "name", "age"))
		// if err != nil {
		// 	log.Fatal(err)
		// }
		//
		// for _, user := range users {
		// 	_, err = stmt.Exec(user.Name, int64(user.Age))
		// 	if err != nil {
		// 		log.Fatal(err)
		// 	}
		// }
		//
		// _, err = stmt.Exec()
		// if err != nil {
		// 	log.Fatal(err)
		// }
		//
		// err = stmt.Close()
		// if err != nil {
		// 	log.Fatal(err)
		// }
		//
		// err = txn.Commit()
		// if err != nil {
		// 	log.Fatal(err)
		// }

		// TODO: for current feed remove any feeds not in

		// for returning array - but will return only ids, probably not enough
		feed["feedGroups"] = feedGroups
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode(user)
	w.Write(feedData)
}

func apiDeleteFeed(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	_, err := db.Exec(`
		DELETE FROM feed_group_feeds WHERE feed = $1;
	`, id)
	if err != nil {
		fmt.Println("Error removing feed from groups:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(`
		DELETE FROM feeds WHERE id = $1;
	`, id)
	if err != nil {
		fmt.Println("Error deleting feed:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func apiGetFeedGroups(w http.ResponseWriter, r *http.Request) {

	db := getDB()

	rows, err := db.Query(`
		SELECT g.id, max(g.name), jsonb_agg(f.id), jsonb_agg(f.data)
		FROM feed_groups g
		LEFT JOIN feed_group_feeds j ON g.id = j.feed_group
		LEFT JOIN feeds f ON f.id = j.feed
		GROUP BY g.id;
	`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	data := make([]interface{}, 0, 10)

	for rows.Next() {
		var (
			id    string
			name  string
			fids  []*string
			feeds []interface{}
		)
		feedGroup := make(map[string]interface{})
		err = rows.Scan(&id, &name, &fids, &feeds)
		if err != nil {
			fmt.Println("ERROR:", err)
			panic(err)
		}
		for i, feed := range feeds {
			if fids[i] == nil {
				continue
			}
			feed.(map[string]interface{})["id"] = *fids[i]
			delete(feed.(map[string]interface{}), "feedGroups") // compensate for incorrect data
		}
		feedGroup["id"] = id
		feedGroup["name"] = name
		if len(feeds) == 0 || feeds[0] == nil {
			feedGroup["feeds"] = make([]interface{}, 0)
		} else {
			feedGroup["feeds"] = []interface{}(feeds)
		}
		data = append(data, feedGroup)
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func apiGetFeedGroup(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	var (
		name  string
		fids  []DBText
		feeds []interface{}
	)

	err := db.QueryRow(`
		SELECT max(g.name), jsonb_agg(f.id), jsonb_agg(f.data)
		FROM feed_groups g
		LEFT JOIN feed_group_feeds j ON g.id = j.feed_group
		LEFT JOIN feeds f ON f.id = j.feed
		WHERE g.id = $1;
	`, id).Scan(&name, &fids, &feeds)
	if err != nil {
		if err == ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Println("db error: get feed group error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	feedGroup := make(map[string]interface{})
	for i, feed := range feeds {
		if fids[i].Status|DBStatusNull > 0 {
			continue
		}
		feed.(map[string]interface{})["id"] = fids[i].String
		delete(feed.(map[string]interface{}), "feedGroups") // compensate for incorrect data
	}
	feedGroup["id"] = id
	feedGroup["name"] = name
	if feeds == nil || len(feeds) == 0 || feeds[0] == nil {
		feedGroup["feeds"] = make([]interface{}, 0)
	} else {
		feedGroup["feeds"] = []interface{}(feeds)
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(feedGroup)
}

func apiPostFeedGroup(w http.ResponseWriter, r *http.Request) {

	var feedGroup struct {
		Name  string   `json:"name"`
		Feeds []string `json:"feeds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&feedGroup); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	uid, err := uuid.NewV4()
	if err != nil {
		apiLogf(r, "error generating uuid: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	id := uid.String()

	db := getDB()

	_, err = db.Exec(`
		INSERT INTO feed_groups (id, name)
		VALUES($1, $2)
	`, id, feedGroup.Name)
	if err != nil {
		apiLogf(r, "error inserting into db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		apiLogf(r, "begin failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	stmt, err := tx.Prepare("post_feed_group_feeds", `
		INSERT INTO feed_group_feeds (feed_group, feed) VALUES($1, $2)
		ON CONFLICT DO NOTHING;
	`)
	if err != nil {
		apiLogf(r, "db prepare statement %v failed: %v", stmt.Name, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for _, fid := range feedGroup.Feeds {
		_, err = tx.Exec(stmt.Name, id, fid)
		if err != nil {
			apiLogf(r, "db exec statement %v failed: %v", stmt.Name, err)
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		apiLogf(r, "commit to db failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	apiGetFeedGroup(w, r, id)
}

func apiPatchFeedGroup(w http.ResponseWriter, r *http.Request, id string) {

	var feedGroup struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Feeds []string `json:"feeds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&feedGroup); err != nil {
		apiLogf(r, "error decoding input JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if feedGroup.ID != id && len(feedGroup.ID) > 0 {
		apiLogf(r, "path ID and payload ID mismatch: %v != %v", id, feedGroup.ID)
		w.WriteHeader(http.StatusBadRequest)
		return
		// } else if len(feedGroup.ID) == 0 {
		// 	feedGroup.ID = id
	}

	db := getDB()

	if len(feedGroup.Name) > 0 {
		_, err := db.Exec(`
			UPDATE feed_groups SET name = $2 WHERE id = $1
		`, id, feedGroup.Name)
		if err != nil {
			apiLogf(r, "error updating db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	tx, err := db.Begin()
	if err != nil {
		apiLogf(r, "begin failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	stmt, err := tx.Prepare("post_feed_group_feeds", `
		INSERT INTO feed_group_feeds (feed_group, feed) VALUES($1, $2)
		ON CONFLICT DO NOTHING;
	`)
	if err != nil {
		apiLogf(r, "db prepare statement %v failed: %v", stmt.Name, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(`
		DELETE FROM feed_group_feeds WHERE feed_group = $1 AND feed != ALL($2);
	`, id, feedGroup.Feeds)
	if err != nil {
		apiLogf(r, "db exec failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, fid := range feedGroup.Feeds {
		_, err = tx.Exec(stmt.Name, id, fid)
		if err != nil {
			apiLogf(r, "db exec statement %v failed: %v", stmt.Name, err)
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		apiLogf(r, "commit to db failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	apiGetFeedGroup(w, r, id)
}

func apiDeleteFeedGroup(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	_, err := db.Exec(`
		DELETE FROM feed_group_feeds WHERE feed_group = $1;
	`, id)
	if err != nil {
		apiLogf(r, "error removing feeds from feed group: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(`
		DELETE FROM feed_groups WHERE id = $1;
	`, id)
	if err != nil {
		apiLogf(r, "error removing feed group: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func apiFeedGroups(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) > 0 && len(parts[0]) == 0 {
		parts = parts[1:]
	}

	if len(parts) == 0 || parts[0] != "feedGroups" {
		panic("invalid path prefx") // TODO: fixme
	}

	parts = parts[1:]

	if len(parts) == 0 {

		if r.Method == "GET" {

			apiGetFeedGroups(w, r)

		} else if r.Method == "POST" {

			apiPostFeedGroup(w, r)
		}
	} else if len(parts) > 0 {

		id := parts[0]
		parts = parts[1:]

		if len(parts) == 0 {

			if r.Method == "GET" {

				apiGetFeedGroup(w, r, id)

			} else if r.Method == "PATCH" {

				apiPatchFeedGroup(w, r, id)

			} else if r.Method == "DELETE" {

				apiDeleteFeedGroup(w, r, id)
			}
		}
	}
}

func apiLiveFeedMediaItems(w http.ResponseWriter, r *http.Request) {

	qs := r.URL.Query()

	var dt *time.Time = nil
	var before *time.Time = nil
	var after *time.Time = nil

	if values := qs["dt"]; len(values) > 0 {
		fmt.Println(values[len(values)-1])
		dt = parseDatetime(values[len(values)-1], nil)
	} else {
		dt = parseDatetime("now", nil)
	}
	dtutc := dt.UTC()
	dt = &dtutc

	if values := qs["margin"]; len(values) > 0 {
		value := values[len(values)-1]
		// var err error
		_before, err := timeSubtractDurationString(*dt, value)
		if err != nil {
			apiLogf(r, "invalid value '%v' for query argument 'margin': %v", value, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_after, err := timeAddDurationString(*dt, value)
		if err != nil {
			apiLogf(r, "invalid value '%v' for query argument 'margin': %v", value, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		before = &_before
		after = &_after
	}

	if values := qs["before"]; len(values) > 0 {
		value := values[len(values)-1]
		// var err error
		_before, err := timeSubtractDurationString(*dt, value)
		if err != nil {
			apiLogf(r, "invalid value '%v' for query argument 'before': %v", value, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		before = &_before
	}

	if values := qs["after"]; len(values) > 0 {
		value := values[len(values)-1]
		// var err error
		_after, err := timeAddDurationString(*dt, value)
		if err != nil {
			apiLogf(r, "invalid value '%v' for query argument 'after': %v", value, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		after = &_after
	}

	if before == nil {
		// var err error
		_before, _ := timeSubtractDurationString(*dt, "24h")
		before = &_before
	}

	if after == nil {
		// var err error
		_after, _ := timeAddDurationString(*dt, "24h")
		after = &_after
	}

	db := getDB()

	sql := `
		SELECT id feed, COALESCE(t2.ids, ARRAY[]::TEXT[]) ids, COALESCE(t2.dts, ARRAY[]::TIMESTAMP[]) dts,
			COALESCE(t2.topics, ARRAY[]::TEXT[]) topics,
			COALESCE(t2.topic_weights, ARRAY[]::DOUBLE PRECISION[]) topic_weights,
			data FROM feeds t1
		LEFT JOIN (
			SELECT /*count(*) rate,*/ feed, array_agg(id) ids, array_agg(datetime) dts, array_agg(topics) topics, array_agg(topic_weights) topic_weights FROM (
				SELECT ni.feed, ni.id, ni.datetime, ni.topics, ni.topic_weights
				FROM news_items ni
				WHERE
				ni.feed = ANY(SELECT id FROM feeds WHERE (data->>'live')::boolean = TRUE) AND
				ni.datetime >= $1 AND ni.datetime <= $2
				GROUP BY ni.feed, ni.id
				ORDER BY ni.feed, ni.datetime ASC
			) t
			GROUP BY t.feed
		) t2 ON id = t2.feed
		WHERE (data->>'live')::boolean = TRUE
	`
	fmt.Println("SQL:", sql)
	fmt.Println("ARGS:", before, after)

	rows, err := db.Query(sql, before, after)
	if err != nil {
		apiLogf(r, "error executing query: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type NewsItem struct {
		ID           string    `json:"id"`
		DateTime     time.Time `json:"timeAdded"`
		Start        int64     `json:"start"`
		Topics       []string  `json:"topics"`
		TopicWeights []float64 `json:"topic_weights"`
	}

	type FeedData struct {
		ID        string                 `json:"feedID"`
		NewsItems []NewsItem             `json:"newsItems"`
		Data      map[string]interface{} `json:"feed"`
		DateTime  int64                  `json:"datetime"`
		Start     int64                  `json:"start"`
		End       int64                  `json:"end"`
	}

	result := make([]FeedData, 0, 100)

	for rows.Next() {
		var (
			feed         FeedData
			newsItems    []string
			timestamps   []time.Time
			data         string
			topics       pgtype.VarcharArray // multi-dimensional array of topics per item
			topicWeights pgtype.Float8Array  // multi-dimensional array of topic weights per item
		)
		err = rows.Scan(&feed.ID, &newsItems, &timestamps, &topics, &topicWeights, &data)
		if err != nil {
			apiLogf(r, "error getting row values: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// fmt.Println(topics.Dimensions)
		// fmt.Println(len(topics.Elements))
		// fmt.Println(topics.Status == pgtype.Present)
		// fmt.Println(topics.Elements[0])
		// fmt.Println(topicWeights)
		// fmt.Println(topicWeights.Dimensions)

		json.Unmarshal([]byte(data), &feed.Data)

		for i, ts := range timestamps {
			var t = make([]string, 0, 10)
			var tw = make([]float64, 0, 10)
			for _, topic := range topics.Elements[10*i : 10*(i+1)] {
				t = append(t, topic.String)
			}
			for _, weight := range topicWeights.Elements[10*i : 10*(i+1)] {
				tw = append(tw, weight.Float)
			}
			feed.NewsItems = append(feed.NewsItems, NewsItem{ID: newsItems[i], DateTime: ts, Start: ts.Unix(), Topics: t, TopicWeights: tw})
		}

		if feed.NewsItems == nil {
			feed.NewsItems = make([]NewsItem, 0)
		}

		feed.DateTime = dt.Unix()
		feed.Start = before.Unix()
		feed.End = after.Unix()

		result = append(result, feed)
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func apiFeeds(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path == "/feeds" && r.Method == "GET" {

		apiGetFeeds(w, r)

	} else if r.URL.Path == "/feeds" && r.Method == "POST" {

		apiPostFeed(w, r)

	} else if r.URL.Path == "/feeds/live/items" /*&& r.Method == "POST"*/ {

		apiLiveFeedMediaItems(w, r)

	} else if strings.HasPrefix(r.URL.Path, "/feeds/") {

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/feeds/"), "/")
		// fmt.Println(parts)

		id := parts[0]

		if len(parts) == 1 {

			if r.Method == "GET" && id == "feedTypes" {

				apiGetFeedTypes(w, r)

			} else if r.Method == "PATCH" {

				apiUpdateFeed(w, r, id)

			} else if r.Method == "DELETE" {

				apiDeleteFeed(w, r, id)

			}
		} else if len(parts) == 2 && parts[1] == "trending" {

			q := r.URL.Query()
			q.Set("feed", id)
			r.URL.RawQuery = q.Encode()

			if r.Method == "GET" {
				apiGetTrending(w, r)
				// apiGetQueryTrending(w, r, "all")
			}
		}
	}
}
