package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
	"strconv"
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

type HistoryReport struct {
	UserID      int
	SegmentName string
	Action      string
	DateTime    string
}

type ReportRequest struct {
	Year  int `json:"year"`
	Month int `json:"month"`
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

	//create table History
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS history (id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES users (id), segment_id INTEGER REFERENCES segments (id), action TEXT, created_at TIMESTAMP WITH TIME ZONE)")
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
	router.HandleFunc("/segments/{segment-name}", deleteSegment(db)).Methods("DELETE")

	//user_segment api
	router.HandleFunc("/user-segments", getUsersSegments(db)).Methods("GET")
	router.HandleFunc("/user-segments", createUsersSegments(db)).Methods("POST")
	router.HandleFunc("/user-segments/{user-id}", getUserSegments(db)).Methods("GET")

	//get CSV report
	router.HandleFunc("/csv-report", getCsvReport(db)).Methods("GET")

	//start server
	log.Fatal(http.ListenAndServe(":8000", jsonContentTypeMiddleware(router)))
}

func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// Api response constructor
func apiResponseConstructor(status string, message string) ApiResponse {
	var apiResponse ApiResponse
	apiResponse.Status = status
	apiResponse.Message = message
	return apiResponse
}

// Report API
func getCsvReport(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var reportRequest ReportRequest
		json.NewDecoder(r.Body).Decode(&reportRequest)

		rows, err := db.Query("SELECT history.user_id, segments.name, history.action, history.created_at FROM history INNER JOIN segments ON segments.id = history.segment_id WHERE EXTRACT(year FROM history.created_at) = $1 AND EXTRACT(month FROM history.created_at) = $2", reportRequest.Year, reportRequest.Month)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := apiResponseConstructor("error", "Something went wrong!")
			log.Println(err)
			json.NewEncoder(w).Encode(response)
			return
		}
		defer rows.Close()

		file, err := os.Create(fmt.Sprintf("report-%d-%d.csv", reportRequest.Year, reportRequest.Month))
		defer file.Close()
		if err != nil {
			log.Fatalln("failed to create/open file", err)
		}

		writer := csv.NewWriter(file)
		defer writer.Flush()

		row := []string{"User", "Action", "Segment", "Date"}
		if err := writer.Write(row); err != nil {
			log.Fatalln("error writing record to file", err)
		}

		for rows.Next() {
			var report HistoryReport

			if err := rows.Scan(&report.UserID, &report.SegmentName, &report.Action, &report.DateTime); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response := apiResponseConstructor("error", "Something went wrong!")
				log.Println(err)
				json.NewEncoder(w).Encode(response)
				return
			}

			row := []string{strconv.Itoa(report.UserID), report.Action, report.SegmentName, report.DateTime}
			if err := writer.Write(row); err != nil {
				log.Fatalln("error writing record to file", err)
			}
		}

		if err := rows.Err(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := apiResponseConstructor("error", "Something went wrong!")
			log.Println(err)
			json.NewEncoder(w).Encode(response)
			return
		}

		w.Write([]byte(""))

		//todo: вернуть ссылку
		//json.NewEncoder(w).Encode(file.Name())
	}
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
				w.WriteHeader(http.StatusInternalServerError)
				response := apiResponseConstructor("error", "Something went wrong!")
				log.Println(err)
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusCreated)
			response := apiResponseConstructor("success", fmt.Sprintf("Segment '%s' was added", segment.Name))
			json.NewEncoder(w).Encode(response)
			return
		} else {
			response := apiResponseConstructor("error", fmt.Sprintf("Segment '%s' already exist!", segment.Name))
			json.NewEncoder(w).Encode(response)
			return
		}
	}
}

// delete Segment (DONE)
func deleteSegment(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		name := vars["segment-name"]

		var segment Segment

		err := db.QueryRow("SELECT name FROM segments WHERE name LIKE $1", name).Scan(&segment.Name)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			response := apiResponseConstructor("error", fmt.Sprintf("Segment '%s' does not exist!", name))
			json.NewEncoder(w).Encode(response)
		} else {
			_, err := db.Exec("DELETE FROM segments WHERE name LIKE $1", name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response := apiResponseConstructor("error", "Something went wrong!")
				log.Println(err)
				json.NewEncoder(w).Encode(response)
				return
			}
			response := apiResponseConstructor("success", fmt.Sprintf("Segment '%s' was deleted!", segment.Name))
			json.NewEncoder(w).Encode(response)
		}
	}
}

// USERS API
// get all Users (DONE)
func getUsers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT * FROM users")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := apiResponseConstructor("error", "Something went wrong!")
			log.Println(err)
			json.NewEncoder(w).Encode(response)
			return
		}
		defer rows.Close()

		users := []User{}
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Name); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response := apiResponseConstructor("error", "Something went wrong!")
				log.Println(err)
				json.NewEncoder(w).Encode(response)
				return
			}
			users = append(users, u)
		}
		if err := rows.Err(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := apiResponseConstructor("error", "Something went wrong!")
			log.Println(err)
			json.NewEncoder(w).Encode(response)
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
			w.WriteHeader(http.StatusInternalServerError)
			response := apiResponseConstructor("error", "Something went wrong!")
			log.Println(err)
			json.NewEncoder(w).Encode(response)
			return
		}
		response := apiResponseConstructor("success", fmt.Sprintf("User '%s' was added!", u.Name))
		json.NewEncoder(w).Encode(response)
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
			w.WriteHeader(http.StatusNotFound)
			response := apiResponseConstructor("error", fmt.Sprintf("User '%s' does not exist!", id))
			json.NewEncoder(w).Encode(response)
		} else {
			_, err := db.Exec("DELETE FROM users WHERE id = $1", id)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response := apiResponseConstructor("error", "Something went wrong!")
				log.Println(err)
				json.NewEncoder(w).Encode(response)
				return
			}
			response := apiResponseConstructor("success", fmt.Sprintf("User '%d' was deleted!", u.ID))
			json.NewEncoder(w).Encode(response)
		}
	}
}

// USER-SEGMENTS API
// create user segments
func createUsersSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userSegment UserSegments
		json.NewDecoder(r.Body).Decode(&userSegment)

		responses := []ApiResponse{}

		errUser := db.QueryRow("SELECT id FROM users WHERE id = $1", userSegment.UserID).Scan(&userSegment.UserID)
		if errors.Is(errUser, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			response := apiResponseConstructor("error", fmt.Sprintf("User '%d' does not exist!", userSegment.UserID))
			json.NewEncoder(w).Encode(response)
			return
		}

		//add user segments
		for _, segment := range userSegment.Segments {

			errSegment := db.QueryRow("SELECT name FROM segments WHERE name LIKE $1", segment).Scan(&segment)

			if errors.Is(errSegment, sql.ErrNoRows) {
				response := apiResponseConstructor("error", fmt.Sprintf("Segment '%s' does not exist!", segment))
				responses = append(responses, response)
				continue
			}

			err := db.QueryRow("SELECT user_id FROM user_segments WHERE user_id = $1 AND segment_id IN (SELECT id FROM segments WHERE name LIKE $2)", userSegment.UserID, segment).Scan(&userSegment.UserID)

			if errors.Is(err, sql.ErrNoRows) {
				_, err := db.Exec("INSERT INTO user_segments (user_id, segment_id) SELECT $1, id FROM segments WHERE name LIKE $2", userSegment.UserID, segment)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					response := apiResponseConstructor("error", "Something went wrong!")
					log.Println(err)
					json.NewEncoder(w).Encode(response)
					return
				}

				_, errHistory := db.Exec("INSERT INTO history (user_id, segment_id, action, created_at) SELECT $1, id, 'add', NOW() FROM segments WHERE name LIKE $2", userSegment.UserID, segment)
				if errHistory != nil {
					log.Println(err)
					continue
				}

				response := apiResponseConstructor("success", fmt.Sprintf("Segment '%s' for user '%d' was added", segment, userSegment.UserID))
				responses = append(responses, response)
				continue
			} else {
				response := apiResponseConstructor("error", fmt.Sprintf("Segment '%s' for user '%d' already exist!", segment, userSegment.UserID))
				responses = append(responses, response)
				continue
			}

		}

		//delete user segments
		for _, segmentDel := range userSegment.SegmentsForDelete {
			errSegment := db.QueryRow("SELECT name FROM segments WHERE name LIKE $1", segmentDel).Scan(&segmentDel)

			if errors.Is(errSegment, sql.ErrNoRows) {
				response := apiResponseConstructor("error", fmt.Sprintf("Segment '%s' does not exist!", segmentDel))
				responses = append(responses, response)
				continue
			}

			err := db.QueryRow("SELECT user_id FROM user_segments WHERE user_id = $1 AND segment_id IN (SELECT id FROM segments WHERE name LIKE $2)", userSegment.UserID, segmentDel).Scan(&userSegment.UserID)

			if errors.Is(err, sql.ErrNoRows) {
				response := apiResponseConstructor("error", fmt.Sprintf("Thete is not segment '%s' for user '%d'", segmentDel, userSegment.UserID))
				responses = append(responses, response)
				continue
			} else {
				_, err := db.Exec("DELETE FROM user_segments WHERE user_id = $1 AND segment_id IN (SELECT id FROM segments WHERE name LIKE $2)", userSegment.UserID, segmentDel)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					response := apiResponseConstructor("error", "Something went wrong!")
					log.Println(err)
					json.NewEncoder(w).Encode(response)
					return
				}

				_, errHistory := db.Exec("INSERT INTO history (user_id, segment_id, action, created_at) SELECT $1, id, 'del', NOW() FROM segments WHERE name LIKE $2", userSegment.UserID, segmentDel)
				if errHistory != nil {
					log.Println(err)
					continue
				}

				response := apiResponseConstructor("success", fmt.Sprintf("Segment '%s' for user '%d' was deleted", segmentDel, userSegment.UserID))
				responses = append(responses, response)
				continue
			}
		}
		json.NewEncoder(w).Encode(responses)
	}
}

// get all segments of users (DONE)
func getUsersSegments(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT user_segments.user_id AS ID, STRING_AGG(COALESCE(segments.name,''), ',') AS name FROM user_segments LEFT JOIN segments ON segments.id = user_segments.segment_id GROUP BY user_segments.user_id")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := apiResponseConstructor("error", "Something went wrong!")
			log.Println(err)
			json.NewEncoder(w).Encode(response)
			return
		}
		defer rows.Close()

		userSegmentsResponse := []UserSegmentForResponse{}
		for rows.Next() {
			var u UserSegmentForResponse
			if err := rows.Scan(&u.ID, &u.Name); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response := apiResponseConstructor("error", "Something went wrong!")
				log.Println(err)
				json.NewEncoder(w).Encode(response)
				return
			}
			userSegmentsResponse = append(userSegmentsResponse, u)
		}

		if err := rows.Err(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			response := apiResponseConstructor("error", "Something went wrong!")
			log.Println(err)
			json.NewEncoder(w).Encode(response)
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
			w.WriteHeader(http.StatusNotFound)
			response := apiResponseConstructor("error", fmt.Sprintf("Segments for user '%s' does not exist!", id))
			json.NewEncoder(w).Encode(response)
		} else {
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				response := apiResponseConstructor("error", "Something went wrong!")
				log.Println(err)
				json.NewEncoder(w).Encode(response)
				return
			}
			json.NewEncoder(w).Encode(u)
		}
	}
}
