package main

import (
	"database/sql"
	"encoding/json"
	"errors"
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
	ID                int      `json:"id"`
	UserID            int      `json:"user-id"`
	Segments          []string `json:"segments"`
	SegmentsForDelete []string `json:"segments-for-delete"`
}

type UserSegmentForResponse struct {
	ID   int    `json:"user-id"`
	Name string `json:"segment-names"`
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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS user_segments (id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES users (id) ON DELETE CASCADE, segment_id INTEGER REFERENCES segments (id) ON DELETE CASCADE)")
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

		_, err := db.Exec("INSERT INTO segments (name) VALUES ($1) ON CONFLICT DO NOTHING RETURNING id", segment.Name)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(fmt.Sprintf("Segment '%s' already exist!", segment.Name))
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

		err := db.QueryRow("SELECT name FROM segments WHERE name LIKE $1", segment.Name).Scan(&segment.Name)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(fmt.Sprintf("Segment '%s' does not exist!", segment.Name))
		} else if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode("ERROR")
			return
		} else {
			_, err := db.Exec("DELETE FROM segments WHERE name LIKE $1", segment.Name)
			if err != nil {
				//FIX
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode("ERROR")
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

		for _, segment := range userSegment.Segments {
			json.NewEncoder(w).Encode(fmt.Sprintf("SEGMENT '%s' '%d'", segment, userSegment.UserID))
			_, err := db.Exec("INSERT INTO user_segments (user_id, segment_id) SELECT $1, id FROM segments WHERE name LIKE $2", userSegment.UserID, segment)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(fmt.Sprintf("ERROR '%s' '%d'", userSegment.Segments, userSegment.UserID))
				return
			}
		}
		json.NewEncoder(w).Encode(fmt.Sprintf("Segments '%s' was added to user '%d'", userSegment.Segments, userSegment.UserID))

		for _, segmentDel := range userSegment.SegmentsForDelete {
			_, err := db.Exec("DELETE FROM user_segments WHERE user_id = $1 AND segment_id IN (SELECT id FROM segments WHERE name LIKE $2)", userSegment.UserID, segmentDel)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(fmt.Sprintf("ERROR '%s' '%d'", userSegment.Segments, userSegment.UserID))
				return
			}
		}
		json.NewEncoder(w).Encode(fmt.Sprintf("Segments '%s' was deleted to user '%d'", userSegment.SegmentsForDelete, userSegment.UserID))

	}
}

// get all segments of users (ALMOST DONE)
func getUsersSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT user_segments.user_id AS ID, STRING_AGG(COALESCE(segments.name,''), ',') AS name FROM user_segments LEFT JOIN segments ON segments.id = user_segments.segment_id GROUP BY user_segments.user_id")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		userSegmentsResponse := []UserSegmentForResponse{}
		for rows.Next() {
			var u UserSegmentForResponse
			if err := rows.Scan(&u.ID, &u.Name); err != nil {
				log.Fatal(err)
			}
			userSegmentsResponse = append(userSegmentsResponse, u)
		}

		if err := rows.Err(); err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(w).Encode(userSegmentsResponse)
	}
}

// get user segments by user_id
func getUserSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["user-id"]

		var u UserSegmentForResponse
		err := db.QueryRow("SELECT user_segments.user_id AS ID, STRING_AGG(segments.name, ',') AS name FROM user_segments INNER JOIN segments ON segments.id = user_segments.segment_id WHERE user_segments.user_id = $1 GROUP BY user_segments.user_id", id).Scan(&u.ID, &u.Name)
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
