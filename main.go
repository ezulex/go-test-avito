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

type ApiResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS user_segments (id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES users (id) ON DELETE CASCADE, segment_id INTEGER REFERENCES segments (id) ON DELETE CASCADE, CONSTRAINT unique_pair UNIQUE (user_id, segment_id))")
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
// create Segment (DONE)
func createSegment(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var segment Segment
		json.NewDecoder(r.Body).Decode(&segment)

		err := db.QueryRow("SELECT name FROM segments WHERE name LIKE $1", segment.Name).Scan(&segment.Name)

		if errors.Is(err, sql.ErrNoRows) {
			_, err := db.Exec("INSERT INTO segments (name) VALUES ($1) ON CONFLICT DO NOTHING RETURNING id", segment.Name)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = "Something went wrong!"
				json.NewEncoder(w).Encode(apiResponse)
				return
			}
			var apiResponse ApiResponse
			apiResponse.Status = "success"
			apiResponse.Message = fmt.Sprintf("Segment '%s' was added", segment.Name)
			json.NewEncoder(w).Encode(apiResponse)
		} else {
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = fmt.Sprintf("Segment '%s' already exist!", segment.Name)
			json.NewEncoder(w).Encode(apiResponse)
			return
		}
	}
}

// delete Segment (DONE)
func deleteSegment(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var segment Segment
		json.NewDecoder(r.Body).Decode(&segment)

		err := db.QueryRow("SELECT name FROM segments WHERE name LIKE $1", segment.Name).Scan(&segment.Name)

		if errors.Is(err, sql.ErrNoRows) {
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = fmt.Sprintf("Segment '%s' does not exist!", segment.Name)
			json.NewEncoder(w).Encode(apiResponse)
		} else {
			_, err := db.Exec("DELETE FROM segments WHERE name LIKE $1", segment.Name)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = "Something went wrong!"
				json.NewEncoder(w).Encode(apiResponse)
				return
			}
			var apiResponse ApiResponse
			apiResponse.Status = "success"
			apiResponse.Message = fmt.Sprintf("Segment '%s' was deleted!", segment.Name)
			json.NewEncoder(w).Encode(apiResponse)
		}
	}
}

// USERS API
// get all Users (DONE)
func getUsers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT * FROM users")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = "Something went wrong!"
			json.NewEncoder(w).Encode(apiResponse)
			log.Fatal(err)
			return
		}
		defer rows.Close()

		users := []User{}
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Name); err != nil {
				w.WriteHeader(http.StatusNotFound)
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = "Something went wrong!"
				json.NewEncoder(w).Encode(apiResponse)
				log.Fatal(err)
				return
			}
			users = append(users, u)
		}
		if err := rows.Err(); err != nil {
			w.WriteHeader(http.StatusNotFound)
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = "Something went wrong!"
			json.NewEncoder(w).Encode(apiResponse)
			log.Fatal(err)
			return
		}
		json.NewEncoder(w).Encode(users)
	}
}

// create User (DONE)
func createUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var u User
		json.NewDecoder(r.Body).Decode(&u)

		err := db.QueryRow("INSERT INTO users (name) VALUES ($1) RETURNING id", u.Name).Scan(&u.ID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = "Something went wrong!"
			json.NewEncoder(w).Encode(apiResponse)
			log.Fatal(err)
			return
		}
		var apiResponse ApiResponse
		apiResponse.Status = "success"
		apiResponse.Message = fmt.Sprintf("User '%s' was added!", u.Name)
		json.NewEncoder(w).Encode(apiResponse)
	}
}

// delete User (DONE)
func deleteUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var u User

		err := db.QueryRow("SELECT id FROM users WHERE id = $1", id).Scan(&u.ID)

		if errors.Is(err, sql.ErrNoRows) {
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = fmt.Sprintf("User '%s' does not exist!", id)
			json.NewEncoder(w).Encode(apiResponse)
		} else {
			_, err := db.Exec("DELETE FROM users WHERE id = $1", id)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = "Something went wrong!"
				json.NewEncoder(w).Encode(apiResponse)
				return
			}
			var apiResponse ApiResponse
			apiResponse.Status = "success"
			apiResponse.Message = fmt.Sprintf("User '%d' was deleted!", u.ID)
			json.NewEncoder(w).Encode(apiResponse)
		}
	}
}

// USER-SEGMENTS API
// create user segments
func createUsersSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userSegment UserSegments
		json.NewDecoder(r.Body).Decode(&userSegment)

		errUser := db.QueryRow("SELECT id FROM users WHERE id = $1", userSegment.UserID).Scan(&userSegment.UserID)
		if errors.Is(errUser, sql.ErrNoRows) {
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = fmt.Sprintf("User '%d' does not exist!", userSegment.UserID)
			json.NewEncoder(w).Encode(apiResponse)
			return
		}

		//add user segments
		for _, segment := range userSegment.Segments {

			errSegment := db.QueryRow("SELECT name FROM segments WHERE name LIKE $1", segment).Scan(&segment)

			if errors.Is(errSegment, sql.ErrNoRows) {
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = fmt.Sprintf("Segment '%s' does not exist!", segment)
				json.NewEncoder(w).Encode(apiResponse)
				continue
			}

			err := db.QueryRow("SELECT user_id FROM user_segments WHERE user_id = $1 AND segment_id IN (SELECT id FROM segments WHERE name LIKE $2)", userSegment.UserID, segment).Scan(&userSegment.UserID)

			if errors.Is(err, sql.ErrNoRows) {
				_, err := db.Exec("INSERT INTO user_segments (user_id, segment_id) SELECT $1, id FROM segments WHERE name LIKE $2", userSegment.UserID, segment)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					var apiResponse ApiResponse
					apiResponse.Status = "error"
					apiResponse.Message = "Something went wrong!"
					json.NewEncoder(w).Encode(apiResponse)
					return
				}
				var apiResponse ApiResponse
				apiResponse.Status = "success"
				apiResponse.Message = fmt.Sprintf("Segment '%s' for user '%d' was added", segment, userSegment.UserID)
				json.NewEncoder(w).Encode(apiResponse)
				continue
			} else {
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = fmt.Sprintf("Segment '%s' for user '%d' already exist!", segment, userSegment.UserID)
				json.NewEncoder(w).Encode(apiResponse)
				continue
			}

		}

		//delete user segments
		for _, segmentDel := range userSegment.SegmentsForDelete {
			errSegment := db.QueryRow("SELECT name FROM segments WHERE name LIKE $1", segmentDel).Scan(&segmentDel)

			if errors.Is(errSegment, sql.ErrNoRows) {
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = fmt.Sprintf("Segment '%s' does not exist!", segmentDel)
				json.NewEncoder(w).Encode(apiResponse)
				continue
			}

			err := db.QueryRow("SELECT user_id FROM user_segments WHERE user_id = $1 AND segment_id IN (SELECT id FROM segments WHERE name LIKE $2)", userSegment.UserID, segmentDel).Scan(&userSegment.UserID)

			if errors.Is(err, sql.ErrNoRows) {
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = fmt.Sprintf("Thete is not segment '%s' for user '%d'", segmentDel, userSegment.UserID)
				json.NewEncoder(w).Encode(apiResponse)
				continue
			} else {
				_, err := db.Exec("DELETE FROM user_segments WHERE user_id = $1 AND segment_id IN (SELECT id FROM segments WHERE name LIKE $2)", userSegment.UserID, segmentDel)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					var apiResponse ApiResponse
					apiResponse.Status = "error"
					apiResponse.Message = "Something went wrong!"
					json.NewEncoder(w).Encode(apiResponse)
					return
				}
				var apiResponse ApiResponse
				apiResponse.Status = "success"
				apiResponse.Message = fmt.Sprintf("Segment '%s' for user '%d' was deleted", segmentDel, userSegment.UserID)
				json.NewEncoder(w).Encode(apiResponse)
				continue
			}
		}
	}
}

// get all segments of users (DONE)
func getUsersSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT user_segments.user_id AS ID, STRING_AGG(COALESCE(segments.name,''), ',') AS name FROM user_segments LEFT JOIN segments ON segments.id = user_segments.segment_id GROUP BY user_segments.user_id")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = "Something went wrong!"
			json.NewEncoder(w).Encode(apiResponse)
			log.Fatal(err)
			return
		}
		defer rows.Close()

		userSegmentsResponse := []UserSegmentForResponse{}
		for rows.Next() {
			var u UserSegmentForResponse
			if err := rows.Scan(&u.ID, &u.Name); err != nil {
				w.WriteHeader(http.StatusNotFound)
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = "Something went wrong!"
				json.NewEncoder(w).Encode(apiResponse)
				log.Fatal(err)
				return
			}
			userSegmentsResponse = append(userSegmentsResponse, u)
		}

		if err := rows.Err(); err != nil {
			w.WriteHeader(http.StatusNotFound)
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = "Something went wrong!"
			json.NewEncoder(w).Encode(apiResponse)
			log.Fatal(err)
			return
		}
		json.NewEncoder(w).Encode(userSegmentsResponse)
	}
}

// get user segments by user_id (DONE)
func getUserSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["user-id"]

		var u UserSegmentForResponse

		err := db.QueryRow("SELECT user_segments.user_id AS ID, STRING_AGG(segments.name, ',') AS name FROM user_segments INNER JOIN segments ON segments.id = user_segments.segment_id WHERE user_segments.user_id = $1 GROUP BY user_segments.user_id", id).Scan(&u.ID, &u.Name)

		if errors.Is(err, sql.ErrNoRows) {
			var apiResponse ApiResponse
			apiResponse.Status = "error"
			apiResponse.Message = fmt.Sprintf("Segments for user '%s' does not exist!", id)
			json.NewEncoder(w).Encode(apiResponse)
		} else {
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				var apiResponse ApiResponse
				apiResponse.Status = "error"
				apiResponse.Message = "Something went wrong!"
				json.NewEncoder(w).Encode(apiResponse)
				return
			}
			json.NewEncoder(w).Encode(u)
		}
	}
}
