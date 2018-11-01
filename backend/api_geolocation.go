package main

import (
	"encoding/json"
	"fmt"
	"math"
	// // "log"
	"net/http"
	// "net/url"
	// "reflect"
	// "regexp"
	// "strconv"
	"strings"
	// "time"
	//
	// // pq "github.com/lib/pq"
	// "github.com/satori/go.uuid"
	// "github.com/jackc/pgx/pgtype"
	"github.com/cridenour/go-postgis"
	// "github.com/paulmach/go.geo"
)

func apiGetLocations(w http.ResponseWriter, r *http.Request) {

	db := getDB()

	// rows, err := db.Query(`SELECT id, baseform, GeomFromEWKB(geo)/*ST_AsBinary(geo)*/ FROM entities WHERE geo IS NOT NULL AND ST_AsText(geo) != 'POINT EMPTY';`)
	// rows, err := db.Query(`SELECT id, baseform, geo::text::bytea FROM entities WHERE geo IS NOT NULL AND ST_AsText(geo) != 'POINT EMPTY';`)
	rows, err := db.Query(`SELECT id, baseform, type, geo::text::bytea FROM entities WHERE geo IS NOT NULL;`)
	if err != nil {
		apiLogf(r, "error getting entities from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type GeoPoint struct {
		Longitude float64 `json:"lng"`
		Latitude  float64 `json:"lat"`
	}

	type Item struct {
		ID          string    `json:"id"`
		BaseForm    string    `json:"baseForm"`
		Type        string    `json:"type"`
		GeoLocation *GeoPoint `json:"geoloc"`
	}

	items := make([]interface{}, 0, 10000)
	for rows.Next() {

		var (
			item Item
			// geo      interface{}
			geo postgis.PointS
			// geo *geo.Point
		)

		err = rows.Scan(&item.ID, &item.BaseForm, &item.Type, &geo)
		if err != nil {
			apiLogf(r, "error getting row values: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !math.IsNaN(geo.X) && !math.IsNaN(geo.Y) {
			item.GeoLocation = &GeoPoint{Longitude: geo.X, Latitude: geo.Y}
		}

		// fmt.Println(item)

		items = append(items, item)
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(items)
}

func apiPostGeolocations(w http.ResponseWriter, r *http.Request) {

	qs := r.URL.Query()

	topics := false

	if values, prs := qs["item"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if lastValue == "topic" {
			topics = true
		}
	}

	db := getDB()

	args := []interface{}{}
	// args = append(args, count)
	conditions_sql := ""
	// conditions_where_sql := ""
	// conditions_and_sql := ""
	joins_sql := ""
	range_sql := ""
	with_sql := ""
	var err error

	err, args, joins_sql, conditions_sql, range_sql, with_sql = apiPOSTBodyToQuerySQLParts(w, r, args, false)
	if err != nil {
		apiLogf(r, "error:", err)
		return
	}
	if len(conditions_sql) > 0 {
		// conditions_sql = " AND " + conditions_sql
		conditions_sql = " WHERE " + conditions_sql
	}
	if len(with_sql) > 0 {
		with_sql = " " + with_sql + ","
	}

	var sql string

	if !topics {
		sql = `
			WITH` + with_sql + ` selected_items AS (
				SELECT ni.* FROM news_items_filtered ni
				` + joins_sql + `
				-- WHERE cluster_id IN (
				-- 	SELECT cluster_id FROM (
				-- 		SELECT cluster_id, count(*) counts
				-- 		FROM (
				-- 			SELECT distinct cluster_id, ni.id FROM news_items_filtered ni
				-- 			` + joins_sql + `
				-- 			WHERE cluster_id IS NOT NULL ` + conditions_sql + `
				-- 		) t
				-- 		GROUP BY cluster_id ORDER BY counts DESC
				-- 	) t
				-- )` + conditions_sql + `
				` + conditions_sql + `
			)
			SELECT entities.id, baseform, type, geo::text::bytea
			FROM
			(
				SELECT DISTINCT entities.id id
				FROM selected_items ni
				--FROM news_items_filtered ni
				JOIN news_item_entities nie ON ni.id = nie.news_item
				JOIN entities ON nie.entity_id = entities.id
				-- only entities with geolocation information
				WHERE geo IS NOT NULL AND ST_AsText(geo) != 'POINT EMPTY'
			) t
			JOIN entities ON t.id = entities.id
			WHERE geo IS NOT NULL AND entities.type = 'places' ` + range_sql + `;`
	} else {
		sql = `
			WITH` + with_sql + ` selected_items AS (
				SELECT ni.* FROM news_items ni
				` + joins_sql + `
				-- WHERE cluster_id IN (
				-- 	SELECT cluster_id FROM (
				-- 		SELECT cluster_id, count(*) counts
				-- 		FROM (
				-- 			SELECT distinct cluster_id, ni.id FROM news_items ni
				-- 			` + joins_sql + `
				-- 			WHERE cluster_id IS NOT NULL ` + conditions_sql + `
				-- 		) t
				-- 		GROUP BY cluster_id ORDER BY counts DESC
				-- 	) t
				-- )` + conditions_sql + `
				` + conditions_sql + `
			)
			SELECT entities.id, baseform, type, geo::text::bytea
			FROM
			(
				SELECT entities.id
				FROM (
					SELECT DISTINCT topic FROM selected_items, unnest(selected_items.topics) topic
					--SELECT DISTINCT topic FROM news_items_filtered ni, unnest(ni.topics) topic
				) x
				JOIN entities ON lower(entities.baseform) = lower(topic)
				WHERE geo IS NOT NULL AND ST_AsText(geo) != 'POINT EMPTY'
			) t
			JOIN entities ON t.id = entities.id
			WHERE geo IS NOT NULL
			--AND entities.type = 'places'
			` + range_sql + `;`
	}

	if config.Debug {
		fmt.Println("    SQL:", sql)
		fmt.Println("    ARGS:", args)
	}

	// rows, err := db.Query(`SELECT id, baseform, type, geo::text::bytea FROM entities WHERE geo IS NOT NULL;`)
	rows, err := db.Query(sql, args...)
	if err != nil {
		// apiLogf(r, "error getting entities from db: %v", err)
		apiLogf(r, "error executing query: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type GeoPoint struct {
		Longitude float64 `json:"lng"`
		Latitude  float64 `json:"lat"`
	}

	type Item struct {
		ID          string    `json:"id"`
		BaseForm    string    `json:"baseForm"`
		Type        string    `json:"type"`
		GeoLocation *GeoPoint `json:"geoloc"`
	}

	items := make([]interface{}, 0, 10000)
	for rows.Next() {

		var (
			item Item
			// geo      interface{}
			geo postgis.PointS
			// geo *geo.Point
		)

		err = rows.Scan(&item.ID, &item.BaseForm, &item.Type, &geo)
		if err != nil {
			apiLogf(r, "error getting row values: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !math.IsNaN(geo.X) && !math.IsNaN(geo.Y) {
			item.GeoLocation = &GeoPoint{Longitude: geo.X, Latitude: geo.Y}
		}

		// fmt.Println(item)

		items = append(items, item)
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(items)
}

func apiGeolocation(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) > 0 && len(parts[0]) == 0 {
		parts = parts[1:]
	}

	if len(parts) == 0 || parts[0] != "locations" {
		panic("invalid path prefx") // TODO: fixme
	}

	parts = parts[1:]

	if len(parts) == 0 {

		if r.Method == "GET" {

			apiGetLocations(w, r)

		} else if r.Method == "POST" {

			apiPostGeolocations(w, r)
		}

	} else if len(parts) > 0 {

		id := parts[0]
		parts = parts[1:]

		if len(parts) == 0 {

			if (r.Method == "GET" || r.Method == "POST") && id == "clusters" {

				apiGetClusters(w, r)

			} else if r.Method == "GET" {

				apiGetMediaItem(w, r, id)

			} else if r.Method == "PATCH" {

				apiPatchMediaItem(w, r, id)
				// apiPatchQuery(w, r, id)

			} else if r.Method == "DELETE" {

				// apiDeleteQuery(w, r, id)
			}
		} else {
			part := parts[0]
			parts = parts[1:]

			if len(parts) == 0 {

				if r.Method == "GET" && part == "neighbours" {

					apiGetMediaItemNeighbours(w, r, id)
				}
			}
		}
	}
}
