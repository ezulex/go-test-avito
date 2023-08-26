package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Segment struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type UserSegments struct {
	ID       int      `json:"id"`
	UserID   int      `json:"user-id"`
	Segments []string `json:"segments"`
}

func main() {
	//connection to DB
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	//CREATE TABLES
	//create table Users
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name TEXT)")
	if err != nil {
		log.Fatal(err)
	}

	//create table Segments
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS segments (id SERIAL PRIMARY KEY, name TEXT UNIQUE)")
	if err != nil {
		log.Fatal(err)
	}

	//create table User_segments
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS user_segments (id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES users (id), segment_id INTEGER REFERENCES segments (id))")
	if err != nil {
		log.Fatal(err)
	}

	//create router
	router := mux.NewRouter()
	//user api
	router.HandleFunc("/users", getUsers(db)).Methods("GET")
	router.HandleFunc("/users", createUser(db)).Methods("POST")
	router.HandleFunc("/users/{id}", deleteUser(db)).Methods("DELETE")

	//segment api
	//router.HandleFunc("/segments", getSegments(db)).Methods("GET")
	router.HandleFunc("/segments", createSegment(db)).Methods("POST")
	//router.HandleFunc("/segments/{id}", updateSegment(db)).Methods("PUT")
	router.HandleFunc("/segments", deleteSegment(db)).Methods("DELETE")

	//user_segment api
	router.HandleFunc("/user-segments", getUsersSegments(db)).Methods("GET")
	router.HandleFunc("/user-segments", createUsersSegments(db)).Methods("POST")
	router.HandleFunc("/user-segments/{user-id}", getUserSegments(db)).Methods("GET")

	//start server
	log.Fatal(http.ListenAndServe(":8000", jsonContentTypeMiddleware(router)))
}

func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// SEGMENTS API
// create Segment
func createSegment(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var segment Segment
		json.NewDecoder(r.Body).Decode(&segment)

		err := db.QueryRow("INSERT INTO segments (name) VALUES ($1) ON CONFLICT DO NOTHING RETURNING id", segment.Name).Scan(&segment.ID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode("This segment already exist")
			return
		}
		json.NewEncoder(w).Encode(segment)
	}
}

// delete Segment
func deleteSegment(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var segment Segment
		json.NewDecoder(r.Body).Decode(&segment)

		_, err := db.Exec("SELECT * FROM segments WHERE name LIKE $1", segment.Name)
		if err != nil {
			fmt.Fprintln(os.Stdout, segment.Name)
			fmt.Fprintln(os.Stdout, err)

			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(segment.Name)
			return
		} else {
			_, err := db.Exec("DELETE FROM segments WHERE name LIKE $1", segment.Name)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			json.NewEncoder(w).Encode(fmt.Sprintf("Segment '%s' was deleted", segment.Name))
		}
	}
}

// USER-SEGMENTS API
// create user segments
func createUsersSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userSegment UserSegments
		json.NewDecoder(r.Body).Decode(&userSegment)

		json.NewEncoder(w).Encode(fmt.Sprintf("First '%s' '%d'", userSegment.Segments, userSegment.UserID))

		for _, segment := range userSegment.Segments {
			json.NewEncoder(w).Encode(fmt.Sprintf("SEGMENT '%s' '%d'", segment, userSegment.UserID))
			err := db.QueryRow("INSERT INTO user_segments (user_id, segment_id) SELECT $1, id FROM segments WHERE name LIKE $2", userSegment.UserID, segment).Scan(&userSegment.ID)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(fmt.Sprintf("ERROR '%s' '%d'", userSegment.Segments, userSegment.UserID))
				return
			}
		}
		json.NewEncoder(w).Encode(userSegment)
	}
}

// get all segments of users
func getUsersSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT user_segments.user_id AS ID, segments.name AS name FROM user_segments INNER JOIN segments ON segments.id = user_segments.segment_id")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		users := []User{}
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Name); err != nil {
				log.Fatal(err)
			}
			users = append(users, u)
		}
		if err := rows.Err(); err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(w).Encode(users)
	}
}

// get user segments by user_id
func getUserSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var u User
		err := db.QueryRow("SELECT user_segments.user_id AS ID, segments.name AS name FROM user_segments INNER JOIN segments ON segments.id = user_segments.segment_id WHERE user_segments.user_id = $1", id).Scan(&u.ID, &u.Name)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(u)
	}
}

// USERS API
// get all Users
func getUsers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT * FROM users")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		users := []User{}
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Name); err != nil {
				log.Fatal(err)
			}
			users = append(users, u)
		}
		if err := rows.Err(); err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(w).Encode(users)
	}
}

// create User
func createUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var u User
		json.NewDecoder(r.Body).Decode(&u)

		err := db.QueryRow("INSERT INTO users (name) VALUES ($1) RETURNING id", u.Name).Scan(&u.ID)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(w).Encode(u)
	}
}

// delete User
func deleteUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var u User
		err := db.QueryRow("SELECT * FROM users WHERE id = $1", id).Scan(&u.ID, &u.Name)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			_, err := db.Exec("DELETE FROM users WHERE id = $1", id)
			if err != nil {
				//todo : fix error handling
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode("User deleted")
		}
	}
}
