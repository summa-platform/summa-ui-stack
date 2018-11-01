package main

import (
	"encoding/json"
	"fmt"
	// // "log"
	"net/http"
	// "net/url"
	// "reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	//
	// // pq "github.com/lib/pq"
	// "github.com/satori/go.uuid"
	"github.com/jackc/pgx/pgtype"
)

func apiGetMediaItem(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	var mediaItem map[string]interface{}
	var origin string

	err := db.QueryRow(`
		SELECT data, origin FROM news_items WHERE id = $1;
	`, id).Scan(&mediaItem, &origin)
	if err != nil {
		if err == ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		apiLogf(r, "error getting media item from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if mm, prs := mediaItem["originalMultiMedia"]; prs {
		mm := mm.(map[string]interface{})
		if videoURL, prs := mm["videoURL"]; prs && videoURL != nil {
			videoURL := videoURL.(string)
			if len(videoURL) > 0 && videoURL[0] == '/' {
				// relative URL
				if _, prs := origins[origin]; prs {
					mm["videoURL"] = "/api/video/" + origin + videoURL
				} else {
					apiLogf(r, "media-item origin id %v not in database", origin)
				}
				/*
					if base, prs := origins[origin]; prs {
						base, err := url.Parse(base)
						if err != nil {
							apiLogf(r, "unable to parse origin %v url %v: %v", origin, base, err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						u, err := url.Parse(videoURL)
						if err != nil {
							apiLogf(r, "unable to parse media item relative url %v: %v", videoURL, err)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						mm["videoURL"] = base.ResolveReference(u).String()
					}
				*/
			}
		}
	}

	mediaItem["id"] = id

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(mediaItem)
}

func apiGetMediaItemNeighbours(w http.ResponseWriter, r *http.Request, id string) {

	db := getDB()

	count := 10

	qs := r.URL.Query()

	if values, prs := qs["count"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if len(lastValue) > 0 {
			value, err := strconv.Atoi(lastValue) // last value
			if err != nil {
				apiLogf(r, "error parsing request 'count' query argument: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			count = value
		}
	}

	rows, err := db.Query(`
		-- NOTE: this outer select is required to improve performance of returning item title
		SELECT a.*, b.data#>'{title}'->>'english' title, b.topics FROM (
			(
				--SELECT id, datetime, data#>'{title,english}' title
				--SELECT id, datetime, data#>'{title}'->>'english' title
				--SELECT id, datetime, COALESCE(title, data#>'{title}'->>'english') title
				--SELECT id, datetime, COALESCE(NULLIF(title,''), data#>'{title}'->>'english') title
				SELECT id, datetime --, COALESCE(NULLIF('',''), data#>'{title}'->>'english') title
				FROM news_items
				WHERE datetime <= (SELECT datetime FROM news_items WHERE id = $1)
				AND feed = (SELECT feed FROM news_items WHERE id = $1)
				AND type = (SELECT type FROM news_items WHERE id = $1)
				ORDER BY datetime DESC
				LIMIT $2
			)
			UNION
			(
				--SELECT id, datetime, data#>'{title,english}' title
				--SELECT id, datetime, data#>'{title}'->>'english' title
				--SELECT id, datetime, COALESCE(title, data#>'{title}'->>'english') title
				--SELECT id, datetime, COALESCE(NULLIF(title,''), data#>'{title}'->>'english') title
				SELECT id, datetime
				FROM news_items
				WHERE datetime >= (SELECT datetime FROM news_items WHERE id = $1)
				AND feed = (SELECT feed FROM news_items WHERE id = $1)
				AND type = (SELECT type FROM news_items WHERE id = $1)
				ORDER BY datetime ASC
				LIMIT $2
			)
		) a
		JOIN news_items b ON a.id = b.id
		ORDER BY a.datetime;`, id, count)

	if err != nil {
		apiLogf(r, "error getting media item neighbours from db: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Result struct {
		ID       string     `json:"id"`
		Datetime *time.Time `json:"timeAdded"`
		Title    string     `json:"title"`
		Topics   []string   `json:"topics"`
	}
	results := make([]Result, 0, count*2)
	for rows.Next() {
		var result Result
		err := rows.Scan(&result.ID, &result.Datetime, &result.Title, &result.Topics)
		// err := rows.Scan(&result.ID, &result.Datetime)
		if err != nil {
			apiLogf(r, "error getting media item neighbour data from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		results = append(results, result)
	}

	w.Header()["Content-Type"] = []string{"application/json"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func apiPatchMediaItem(w http.ResponseWriter, r *http.Request, id string) {

	query := r.URL.Query()

	if values, prs := query["set"]; prs && len(values) > 0 {

		value := values[len(values)-1] // last value

		if value == "cluster" {

			var clusterID int64

			// get value from json payload
			if err := json.NewDecoder(r.Body).Decode(&clusterID); err != nil {
				apiLogf(r, "error: invalid request, payload must contain single integer number - cluster id")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// POSTGRES set cluster ID for document ID
			db := getDB()

			_, err := db.Exec(`
				UPDATE mediaItems SET cluster_id = $2 WHERE id = $1;
			`, id, clusterID)
			if err != nil {
				apiLogf(r, "error updating db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		}
	} else {
		apiLogf(r, "error: invalid request, either query argument \"set\" is not set or contains unsupported value")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func apiPostMediaItemClusterBuckets(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query()

	if values, prs := query["set"]; prs && len(values) > 0 {

		value := values[len(values)-1] // last value

		if value == "cluster-buckets" {

			type ClusterBucketUpdate struct {
				clusterIDs []int
				bucketID   int
			}

			var clusterBucketUpdates []ClusterBucketUpdate

			// get value from json payload
			if err := json.NewDecoder(r.Body).Decode(&clusterBucketUpdates); err != nil {
				apiLogf(r, "error: invalid request, payload must contain single integer number - cluster id")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			db := getDB()

			// [[cross1, [mono1, mono2,...], [cross2, [mono10, mono11, ...]], ...]

			for _, cluster_bucket_update := range clusterBucketUpdates {
				_, err := db.Exec(`
					UPDATE mediaItems SET cluster_bucket_id = $2 WHERE cluster_id IN $1;
				`, cluster_bucket_update.clusterIDs, cluster_bucket_update.bucketID)
				if err != nil {
					apiLogf(r, "error updating db: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			w.WriteHeader(http.StatusOK)
		}
	} else {
		apiLogf(r, "error: invalid request, either query argument \"set\" is not set or contains unsupported value")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func apiGetMediaItems(w http.ResponseWriter, r *http.Request) {

	// batch=unclustered, count=100

	query := r.URL.Query()

	if values, prs := query["batch"]; prs && len(values) > 0 {
		batch := values[len(values)-1] // last value
		if batch != "unclustered" {
			apiLogf(r, "error: invalid request, currently only batch=unclustered mode is supported")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		count := 100
		if values, prs = query["count"]; prs && len(values) > 0 {
			var err error
			count, err = strconv.Atoi(values[len(values)-1])
			if err != nil {
				apiLogf(r, "error: invalid request, count parameter must be integer")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		db := getDB()

		limitstr := ""
		if count > 0 {
			limitstr = " LIMIT " + strconv.Itoa(count)
		}

		rows, err := db.Query(`SELECT id, data FROM news_items WHERE cluster_id IS NULL ORDER BY last_changed ASC` + limitstr + `;`)
		if err != nil {
			apiLogf(r, "error getting media items from db: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		mediaItems := make([]interface{}, 0, count)
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
			mediaItem["id"] = itemID

			// export only selected fields, not everything

			// remove some bigger unused properties
			delete(mediaItem, "detectedTopics")
			delete(mediaItem["transcript"].(map[string]interface{})["english"].(map[string]interface{}), "wordTimestampsAndConfidences")
			delete(mediaItem["transcript"].(map[string]interface{})["original"].(map[string]interface{}), "wordTimestampsAndConfidences")

			// --------------
			// out := map[string]interface{}{}
			// out["id"] = itemID
			// titles := mediaItem["title"].(map[string]interface{})
			// title, prs := titles["english"]
			// if !prs || title == nil {
			// 	title = titles["original"]
			// }
			// out["title"] = title
			// // out["title"] = mediaItem["title"].(map[string]interface{})["english"]
			// out["detectedLangCode"] = mediaItem["detectedLangCode"]
			// out["detectedTopics"] = mediaItem["detectedTopics"]
			// out["mediaItemType"] = mediaItem["mediaItemType"]
			// out["sentiment"] = mediaItem["sentiment"]
			// out["timeAdded"] = mediaItem["timeAdded"]
			// out["source"] = mediaItem["source"]
			// data.(map[string]interface{})["id"] = itemID
			// mediaItems = append(mediaItems, out)

			mediaItems = append(mediaItems, mediaItem)
		}

		/*
			var mediaItem map[string]interface{}
			var origin string

			err := db.QueryRow(`
				SELECT data, origin FROM news_items WHERE id = $1;
			`, id).Scan(&mediaItem, &origin)
			if err != nil {
				apiLogf(r, "error getting media item from db: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		*/

		/*
			if mm, prs := mediaItem["originalMultiMedia"]; prs {
				mm := mm.(map[string]interface{})
				if videoURL, prs := mm["videoURL"]; prs && videoURL != nil {
					videoURL := videoURL.(string)
					if len(videoURL) > 0 && videoURL[0] == '/' {
						// relative URL
						if _, prs := origins[origin]; prs {
							mm["videoURL"] = "/api/video/" + origin + videoURL
						} else {
							apiLogf(r, "media-item origin id %v not in database", origin)
						}
					}
				}
			}
		*/

		// mediaItem["id"] = id

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mediaItems)
	}
}

func apiGetClusters(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query()

	group := false
	count := 40
	offset := 0
	sortBy := "size"

	if values, prs := query["count"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if len(lastValue) > 0 {
			value, err := strconv.Atoi(lastValue) // last value
			if err != nil {
				apiLogf(r, "error parsing 'count' query parameter:", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			count = value
		}
	}

	if values, prs := query["offset"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if len(lastValue) > 0 {
			value, err := strconv.Atoi(lastValue) // last value
			if err != nil {
				apiLogf(r, "error parsing 'offset' query parameter:", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			offset = value
		}
	}

	if values, prs := query["group"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if len(lastValue) > 0 {
			group = true
		}
	}

	if values, prs := query["sort"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if lastValue == "last_update" || lastValue == "time" {
			sortBy = "last_update"
		}
	}

	var exclude []string
	if values, prs := query["exclude"]; prs && len(values) > 0 {
		exclude = make([]string, len(values))
		for _, value := range values {
			exclude = append(exclude, value)
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

	if r.Method == "POST" {

		err, args, joins_sql, conditions_sql, range_sql, with_sql = apiPOSTBodyToQuerySQLParts(w, r, args, false)
		if err != nil {
			apiLogf(r, "get clusters error:", err)
			return
		}
		if len(conditions_sql) > 0 {
			conditions_sql = " AND " + conditions_sql
		}
		if len(with_sql) > 0 {
			with_sql = " " + with_sql + ","
		}
	} else {
		if count > 0 && offset > 0 {
			range_sql = "LIMIT $" + strconv.Itoa(len(args)+1)
			args = append(args, count)
			range_sql = range_sql + " OFFSET $" + strconv.Itoa(len(args)+1)
			args = append(args, offset)
		} else if count > 0 {
			range_sql = "LIMIT $" + strconv.Itoa(len(args)+1)
			args = append(args, count)
		} else if offset > 0 {
			range_sql = "OFFSET $" + strconv.Itoa(len(args)+1)
			args = append(args, offset)
		}
	}

	if group {

		sql := `
			WITH` + with_sql + ` selected_items AS (
				SELECT ni.* FROM news_items_filtered ni
				` + joins_sql + `
				WHERE cluster_id IN (
					SELECT cluster_id FROM (
						SELECT cluster_id, count(*) counts
						FROM (
							SELECT DISTINCT cluster_id, ni.id FROM news_items_filtered ni
							` + joins_sql + `
							WHERE cluster_id IS NOT NULL ` + conditions_sql + `
						) t
						GROUP BY cluster_id ORDER BY counts DESC ` + range_sql + `
						--GROUP BY cluster_id ORDER BY counts DESC LIMIT _1
					) t
				)` + conditions_sql + `
			)
			SELECT topic, array_agg(cluster_id) cluster_ids, array_agg(size) sizes, sum(size) topic_size,
			/*array_agg(datetime) datetimes, array_agg(id) ids,*/ jsonb_agg("data") items, array_agg(title) titles FROM (
				SELECT a.*, b.topic, c.data, clusters.title FROM (
					SELECT cluster_id, count(*) size, /*(array_agg(datetime))[count(*)/2+1] datetime,*/ (array_agg(id))[count(*)/2+1] id
					FROM (
						SELECT cluster_id, datetime, id FROM selected_items ORDER BY cluster_id, datetime DESC
					) t GROUP BY cluster_id
				) a
				JOIN (
				-- return cluster_id and topic
				SELECT DISTINCT ON (cluster_id) cluster_id, topic FROM (
					SELECT cluster_id, topic, /*count(topic) count, sum(weight) weight,*/
					--sum(weight)*count(topic) weighted
					--sum(weight) weighted
					count(topic) weighted
					FROM (
						-- select news items and unnest topics and topic weights
						SELECT cluster_id, unnest(topics) topic, unnest(topic_weights) weight FROM selected_items
					) t GROUP BY cluster_id, topic ORDER BY cluster_id, weighted DESc
				) t WINDOW w AS (
					PARTITION BY cluster_id ORDER BY weighted DESC
					-- ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING
				)) b
				ON a.cluster_id = b.cluster_id
				JOIN news_items c ON a.id = c.id
				LEFT JOIN clusters ON a.cluster_id = clusters.id
			) t GROUP BY topic ORDER BY topic_size DESC;`

		if config.Debug {
			fmt.Println("    SQL:", sql)
			fmt.Println("    ARGS:", args)
		}

		rows, err := db.Query(sql, args...)
		if err != nil {
			apiLogf(r, "error executing query: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// topic, cluster_ids, sizes, topic_size, items

		eofs := regexp.MustCompile("[.!?]")

		topicClusters := make([]interface{}, 0, count)
		for rows.Next() {
			var (
				topic        string
				clusterIDs   []int32
				clusterSizes []int64
				topicSize    int
				// items        pgtype.JSONB               // this will return escaped string of psql array from items.Value() in case of psql array of jsonb
				// items        pgtype.BPCharArray         // this will allow to iterate over array of items and unmarshal each to map in case of psql array of jsonb
				items  []map[string]interface{} // in case of jsonb_agg
				titles pgtype.VarcharArray
			)
			err = rows.Scan(&topic, &clusterIDs, &clusterSizes, &topicSize, &items, &titles)
			if err != nil {
				apiLogf(r, "error getting row values: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			/*
				// value, err := items.Value()
				// fmt.Println(value, err, reflect.TypeOf(value))
				for i := int32(0); i < items.Dimensions[0].Length; i++ {
					fmt.Println(items.Elements[i])
					var item interface{}
					items.Elements[i].Scan(&item)
					value, err := items.Elements[i].Value()
					if err != nil {
						apiLogf(r, "error getting row values, unable to parse item data: %v", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					err = json.Unmarshal([]byte(value.(string)), &item)
					if err != nil {
						apiLogf(r, "error unmarshalling value: %v", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					// fmt.Println(item)
					// fmt.Println(reflect.TypeOf(item))
				}
			*/

			// [
			//	{ topic,
			//    size,
			//    clusters: [
			//      id,
			//      size,
			//      title,
			//    ]
			// ]

			// fmt.Println(items)

			topicCluster := map[string]interface{}{}
			topicCluster["topic"] = topic
			topicCluster["size"] = topicSize

			clusters := make([]map[string]interface{}, 0, len(clusterIDs))
			for i := 0; i < len(clusterIDs); i++ {
				var title string
				if t, err := titles.Elements[i].Value(); err == nil && t != nil {
					title = t.(string)
					// fmt.Println("-", title)
				}
				if len(title) == 0 {
					title = items[i]["title"].(map[string]interface{})["english"].(string)
					if strings.HasPrefix(title, "= =") || strings.HasSuffix(title, "-chunk") || strings.HasSuffix(title, " Chunk.") || strings.HasSuffix(title, " Chunk") {
						switch summary := items[i]["summary"].(type) {
						case []interface{}:
							if len(summary) > 0 {
								title = summary[0].(string)
							}
						case []string:
							if len(summary) > 0 {
								title = summary[0]
							}
						case string:
							if len(summary) > 0 {
								title = summary
							}
						}
						sentences := eofs.Split(title, 2)
						if len(sentences[0]) > 0 {
							title = sentences[0]
						} else if len(sentences[1]) > 0 {
							title = sentences[1]
						}
					}
				}
				// if len(title) == 0 {
				// 	title = items[i]["title"].(map[string]interface{})["english"].(string)
				// }
				clusters = append(clusters, map[string]interface{}{
					"id":    clusterIDs[i],
					"size":  clusterSizes[i],
					"title": title,
					// "title": items[i]["title"].(map[string]interface{})["english"],
					// "title": items.([]map[string]interface{})[i]["title"].(map[string]interface{})["english"],
				})
			}

			topicCluster["clusters"] = clusters

			topicClusters = append(topicClusters, topicCluster)
		}

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(topicClusters)

	} else {

		sql := `
			WITH` + with_sql + ` selected_items AS (
				SELECT ni.* FROM news_items_filtered ni
				` + joins_sql + `
				WHERE cluster_id IN (
					SELECT cluster_id FROM (
						SELECT cluster_id, count(*) counts
						FROM (
							SELECT DISTINCT cluster_id, ni.id FROM news_items_filtered ni
							` + joins_sql + `
							WHERE cluster_id IS NOT NULL ` + conditions_sql + `
						) t
						GROUP BY cluster_id ORDER BY counts DESC
					) t
				)` + conditions_sql + `
			)
			SELECT cluster_id, size, total_count, last_update, COALESCE(title, '')
			FROM (
				SELECT t.cluster_id, count(*) size, last_update, count(*) OVER() total_count
				FROM selected_items
				LEFT JOIN (
					SELECT cluster_id, max(last_changed) last_update
					FROM selected_items
					GROUP BY cluster_id
				) t ON t.cluster_id = selected_items.cluster_id
				GROUP BY t.cluster_id, last_update
				ORDER BY ` + sortBy + ` DESC
				` + range_sql + `
			) a
			LEFT JOIN clusters ON cluster_id = clusters.id;`

		if config.Debug {
			fmt.Println("    SQL:", sql)
			fmt.Println("    ARGS:", args)
		}

		rows, err := db.Query(sql, args...)
		if err != nil {
			apiLogf(r, "error executing query: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type Cluster struct {
			ID         int32     `json:"id"`
			Size       int       `json:"size"`
			LastUpdate time.Time `json:"lastUpdate"`
			Title      string    `json:"title"`
		}

		// topicClusters := make([]interface{}, 0, count)
		result := struct {
			TotalCount int       `json:"totalCount"`
			Count      int       `json:"count"`
			Offset     int       `json:"offset"`
			Clusters   []Cluster `json:"clusters"`
		}{Offset: offset, Clusters: make([]Cluster, 0, count)}

		for rows.Next() {
			var cluster Cluster
			err = rows.Scan(&cluster.ID, &cluster.Size, &result.TotalCount, &cluster.LastUpdate, &cluster.Title)
			if err != nil {
				apiLogf(r, "error getting row values: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			result.Count += 1
			result.Clusters = append(result.Clusters, cluster)
		}

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}
}

func apiMediaItems(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) > 0 && len(parts[0]) == 0 {
		parts = parts[1:]
	}

	if len(parts) == 0 || parts[0] != "mediaItems" {
		panic("invalid path prefx") // TODO: fixme
	}

	parts = parts[1:]

	if len(parts) == 0 {

		if r.Method == "GET" {

			apiGetMediaItems(w, r) // ?batch=unclustered&count=100

		} else if r.Method == "POST" {

			apiPostMediaItemClusterBuckets(w, r) // ?set=cluster-buckets
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
