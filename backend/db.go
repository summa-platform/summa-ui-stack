package main

import (
	"fmt"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
	"log"
	"os"
	"time"
)

type DBText = pgtype.Text
type DBValueStatus = pgtype.Status

const DBStatusPresent = pgtype.Present
const DBStatusNull = pgtype.Null

type DB = *pgx.ConnPool
type Rows = *pgx.Rows

var ErrNoRows = pgx.ErrNoRows

var pool *pgx.ConnPool
var poolConfig *pgx.ConnPoolConfig

func getDB() DB {

	if pool != nil {
		return pool
	}

	if poolConfig == nil {
		poolConfig = &pgx.ConnPoolConfig{
			MaxConnections: 5,
			ConnConfig: pgx.ConnConfig{
				User:              config.DBConfig.User,
				Password:          config.DBConfig.Password,
				Host:              config.DBConfig.Host,
				Port:              config.DBConfig.Port,
				Database:          config.DBConfig.Database,
				TLSConfig:         nil,
				UseFallbackTLS:    false,
				FallbackTLSConfig: nil,
				RuntimeParams:     map[string]string{"application_name": config.DBConfig.ApplicationName},
			},
		}
	}

	var last, err error
	count := 0
	for {
		pool, err = pgx.NewConnPool(*poolConfig)
		if err == nil {
			if last != nil {
				fmt.Fprintf(os.Stderr, " connected\n")
			}
			return pool
		}
		newError := last == nil || last.Error() != err.Error()
		if newError {
			// different error than last one, should be reported
			if last != nil {
				fmt.Fprintf(os.Stderr, "\n")
			}
			// fmt.Fprintf(os.Stderr, "Unable to establish connection to database: %v\n", err)
			log.Printf("Unable to establish connection to database: %v\n", err)
		}
		if count >= 10 {
			panic("unable to connect to database in 10 tries")
		}
		time.Sleep(10 * time.Second)
		if newError {
			fmt.Fprintf(os.Stderr, "Reconnecting .")
			last = err
		} else {
			fmt.Fprintf(os.Stderr, ".")
		}
		count++
	}
}

/*
func getDB() DB {
	var err error

	if db != nil {
		err = db.Ping()
		if err == nil {
			return db
		}
	}

	// TODO: implement timeout
	for {
		log.Println("Connecting to database")

		psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbhost, dbport, dbuser, dbpassword, dbname)
		db, err = sql.Open("postgres", psqlInfo)
		if err != nil {
			log.Println("Error opening connection to database")
			continue
		}

		err = db.Ping()
		if err == nil {
			return db
		}

		log.Println("Error pinging database")

		time.Sleep(10 * time.Second)
	}
}
*/

func shutdownDB() {
	getDB().Close()
}

type Origin struct {
	url      string
	videoURL string
}

var origins map[string]Origin

func dbGetOrigins() {

	origins = make(map[string]Origin)

	db := getDB()

	rows, err := db.Query(`
		SELECT name, url, video_url FROM origins;
	`)
	if err != nil {
		fmt.Printf("error getting origins from db: %v\n", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var (
			name     string
			url      string
			videoURL string
		)
		err := rows.Scan(&name, &url, &videoURL)
		if err != nil {
			fmt.Printf("error getting origin value from db: %v\n", err)
			return
		}
		if videoURL == "" {
			url = videoURL
		}
		origins[name] = Origin{url, videoURL}
	}
}

/*
func dbtest() {
	var err error
	var db *sql.DB

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbhost, dbport, dbuser, dbpassword, dbname)
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully connected!")

	rows, err := db.Query("SELECT id, baseform, type FROM entities LIMIT $1", 3)
	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var baseform string
		var typ string
		err = rows.Scan(&id, &baseform, &typ)
		if err != nil {
			// handle this error
			panic(err)
		}
		fmt.Println(id, baseform, typ)
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		panic(err)
	}

	// err = db.QueryRow(`select * from stories`).Scan(&id)
	// if err != nil {
	// 	panic(err)
	// }

		// 	sqlStatement := `
		// INSERT INTO users (age, email, first_name, last_name)
		// VALUES ($1, $2, $3, $4)
		// RETURNING id`
		// 	id := 0
		// 	err = db.QueryRow(sqlStatement, 30, "jon@calhoun.io", "Jonathan", "Calhoun").Scan(&id)
		// 	if err != nil {
		// 		panic(err)
		// 	}
		// 	fmt.Println("New record ID is:", id)
}
*/
