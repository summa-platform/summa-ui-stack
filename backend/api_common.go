package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func apiPOSTBodyToQuerySQLParts(
	w http.ResponseWriter,
	r *http.Request,
	pre_args []interface{},
	enable_geoloc bool,
) (err error, args []interface{}, joins_sql string, conditions_sql string, range_sql string, with_sql string) {

	args = pre_args

	type GeoLocationConstraint struct {
		Latitude  float64 `json:"lat"`
		Longitude float64 `json:"lng"`
		Radius    float64 `json:"radius"`
	}

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

	if err = json.NewDecoder(r.Body).Decode(&query); err != nil {
		err = fmt.Errorf("error parsing request body as JSON: %v", err)
		// apiLogf(r, "error parsing request body as JSON: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var (
		entities  []string
		entityIds []string
		feeds     []string
	)

	for _, entity := range query.Entities {
		switch value := entity.(type) {
		case string:
			entities = append(entities, value) // baseform
		case map[string]interface{}:
			if id, prs := value["id"]; prs {
				entityIds = append(entityIds, id.(string)) // { id: ..., ... }
			}
		}
	}

	if config.Debug {
		apiLogf(r, "debug: query entities: %v", entities)
		apiLogf(r, "debug: query entity ids: %v", entityIds)
	}

	// list of feeds from feed groups
	if len(query.FeedGroups) > 0 {

		db := getDB()

		var rows Rows

		rows, err = db.Query(`
			SELECT DISTINCT feed
			FROM feed_group_feeds WHERE feed_group = ANY($1);
		`, query.FeedGroups)
		if err != nil {
			err = fmt.Errorf("error selecting feed group feeds from DB: %v", err)
			// apiLogf(r, "error selecting feed group feeds from DB: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var feed string
			err = rows.Scan(&feed)
			if err != nil {
				err = fmt.Errorf("error getting row of feed group feed value from DB: %v", err)
				// apiLogf(r, "error getting row of feed group feed value from DB: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			feeds = append(feeds, feed)
		}
	}
	// add single feeds
	if len(query.Feeds) > 0 {
		feeds = append(feeds, query.Feeds...)
	}

	// time formats: timestamp, -NNNh, YYYYmmdd[T]HHMMSS[Z] (UTC)

	now := time.Now().UTC()

	var from *time.Time = nil
	var till *time.Time = nil

	qs := r.URL.Query()

	if values, prs := qs["count"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if len(lastValue) > 0 {
			value, e := strconv.Atoi(lastValue) // last value
			if e != nil {
				err = fmt.Errorf("error parsing request 'count' query argument: %v", e)
				// apiLogf(r, "error parsing request 'count' query argument: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			query.Limit = value
		}
	} else if values, prs := qs["limit"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if len(lastValue) > 0 {
			value, e := strconv.Atoi(lastValue) // last value
			if e != nil {
				err = fmt.Errorf("error parsing request 'limit' query argument: %v", e)
				// apiLogf(r, "error parsing request 'limit' query argument: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			query.Limit = value
		}
	}

	if values, prs := qs["offset"]; prs && len(values) > 0 {
		lastValue := values[len(values)-1]
		if len(lastValue) > 0 {
			value, e := strconv.Atoi(lastValue) // last value
			if e != nil {
				err = fmt.Errorf("error parsing request 'offset' query argument: %v", e)
				// apiLogf(r, "error parsing request 'offset' query argument: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			query.Offset = value
		}
	}

	if values := qs["from"]; len(values) > 0 {
		from = parseDatetime(values[len(values)-1], nil)
	} else {
		from = parseDatetime("-24h", nil) // defaults to -24h
	}

	if values := qs["till"]; len(values) > 0 {
		till = parseDatetime(values[len(values)-1], nil)
	} else {
		till = &now
	}

	if config.Debug {
		apiLogf(r, "debug: from: %v", *from)
		apiLogf(r, "debug: till: %v", *till)
	}

	if query.Till != "" {
		till = parseDatetime(query.Till, nil)
	}

	if query.From != "" {
		from = parseDatetime(query.From, till)
	} else {
		from = parseDatetime("-24", till) // defaults to -24h
	}

	// if query.Limit == 0 {
	// 	query.Limit = 100
	// }

	var clusterID = -1
	if query.ClusterID != nil {
		clusterID = int(query.ClusterID.(float64))
	}

	query.FullTextSearch = fullTextSearchToTSQuery(query.FullTextSearch)

	{
		entities := &entities
		entityIds := &entityIds
		mediaTypes := &query.MediaTypes
		languages := &query.Languages
		fullTextSearch := query.FullTextSearch

		conditions := []string{}
		joins := []string{}
		withs := []string{}

		if (entityIds != nil && len(*entityIds) > 0) || (entities != nil && len(*entities) > 0) || (enable_geoloc && query.GeoLocation != nil) {

			entityConditions := []string{}

			if entityIds != nil && len(*entityIds) > 0 {
				entityConditions = append(entityConditions, "nie.entity_id = ANY($"+strconv.Itoa(len(args)+1)+")")
				args = append(args, *entityIds)
			}

			if entities != nil && len(*entities) > 0 {
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

			if enable_geoloc && query.GeoLocation != nil {
				narg := len(args) + 1
				withs = append(withs, `
					entities_geo AS (
						SELECT id FROM entities
						WHERE ST_DWithin(geo::geography,ST_SetSRID(ST_MakePoint($`+
					strconv.Itoa(narg)+",$"+strconv.Itoa(narg+1)+"),4326)::geography,$"+strconv.Itoa(narg+2)+`)
					)
				`)
				args = append(args, query.GeoLocation.Longitude)
				args = append(args, query.GeoLocation.Latitude)
				args = append(args, query.GeoLocation.Radius)

				join_sql := `JOIN entities_geo e ON e.id = nie.entity_id`
				joins = append(joins, join_sql)
			}
		}

		{
			conditions := []string{}

			if len(feeds) > 0 {
				conditions = append(conditions, "ni.feed = ANY($"+strconv.Itoa(len(args)+1)+")")
				args = append(args, feeds)
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

			conditions_sql := ""
			if len(conditions) > 0 {
				conditions_sql = "WHERE " + strings.Join(conditions, " AND ")
			}

			if len(conditions) > 0 {
				withs = append(withs, `
					news_items_filtered AS (
						SELECT * FROM news_items ni
						`+conditions_sql+`
					)
				`)
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

		if len(joins) > 0 {
			joins_sql = strings.Join(joins, " ")
		}
		if len(conditions) > 0 {
			conditions_sql = strings.Join(conditions, " AND ")
		}
		if len(withs) > 0 {
			with_sql = strings.Join(withs, ",\n")
		}
	}

	if query.Limit > 0 && query.Offset > 0 {
		range_sql = "LIMIT $" + strconv.Itoa(len(args)+1)
		args = append(args, query.Limit)
		range_sql = range_sql + " OFFSET $" + strconv.Itoa(len(args)+1)
		args = append(args, query.Offset)
	} else if query.Limit > 0 {
		range_sql = "LIMIT $" + strconv.Itoa(len(args)+1)
		args = append(args, query.Limit)
	} else if query.Offset > 0 {
		range_sql = "OFFSET $" + strconv.Itoa(len(args)+1)
		args = append(args, query.Offset)
	}

	return
}
