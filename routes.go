package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
    "strings"
    "context"

	jwt "github.com/dgrijalva/jwt-go"
)

type spaHandler struct {
    staticPath string
    indexPath  string
}

func (s *server) routes() {
    api := s.router.PathPrefix("/api").Subrouter()
    api.Use(s.withAuth)
	api.Handle("/reviews", s.handleReviewsGet()).Methods("GET")
	api.HandleFunc("/reviews/{id}", s.handleReviewsGet()).Methods("GET")
	api.HandleFunc("/reviews", s.handleReviewPost()).Methods("POST")
	api.HandleFunc("/auth", s.handleAuthLogin()).Methods("POST")
	api.HandleFunc("/me", s.handleUserGet()).Methods("GET")

    s.router.PathPrefix("/").Handler(s.handlerSPA("build", "index.html"))
}

func (s *server) handleReviewsGet() http.HandlerFunc {
    return func (w http.ResponseWriter, r *http.Request) {
        var reviews []Review
        s.db.Find(&reviews)
        json.NewEncoder(w).Encode(reviews)
    }
}

func (s *server) handleReviewGet() http.HandlerFunc {
    return func (w http.ResponseWriter, r *http.Request) {
        var review Review
        s.db.Where("ID = ?").Find(&review)
        json.NewEncoder(w).Encode(review)
    }
}

func (s *server) handleReviewPost() http.HandlerFunc {
    return func (w http.ResponseWriter, r *http.Request) {
        var review Review
        err := json.NewDecoder(r.Body).Decode(&review)
        if err != nil {
            fmt.Fprintf(w, "Couldn't decode review")
        } else {
            if review.User.Username == r.Context().Value("username") {
                s.db.Create(&review)
                json.NewEncoder(w).Encode(review)
            } else {
                fmt.Fprintf(w, "That user is not you")
            }
        }
    }
}

func (s *server) handleAuthLogin() http.HandlerFunc {
    type Credentials struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }

    return func (w http.ResponseWriter, r *http.Request) {
        var credentials Credentials
        err := json.NewDecoder(r.Body).Decode(&credentials)
        if err != nil {
            fmt.Fprintf(w, "couldn't decode")
            return
        }

        token := jwt.New(jwt.SigningMethodHS256)
        claims := token.Claims.(jwt.MapClaims)

        claims["username"] = credentials.Username

        tokenString, err := token.SignedString(s.jwtSecretKey)

        if err != nil {
            fmt.Fprintf(w, "couldn't sign token")
            return
        }

        fmt.Fprintf(w, tokenString)
    }
}

func (s *server) handleUserGet() http.HandlerFunc {
    return func (w http.ResponseWriter, r *http.Request) {
	    json.NewEncoder(w).Encode(User{ ID: 0, Username: "angusjf", Image: "" })
    }
}

func (s *server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        auth := r.Header.Get("Authorization")
		if auth == "" {
            // no token, just go to endpoint anyway
            next.ServeHTTP(w, r)
            return
        }

        trimmed := strings.TrimPrefix("Bearer ", auth)

        token, err := jwt.Parse(trimmed, func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("Error parsing token")
            } else {
                return s.jwtSecretKey, nil
            }
        })

        if err != nil {
            fmt.Fprintf(w, err.Error())
        }

        if token.Valid {
            claims := token.Claims.(jwt.MapClaims)
            ctx := context.WithValue(r.Context(), "username", claims["username"])
            next.ServeHTTP(w, r.Clone(ctx))
        }
	})
}

func (s *server) handlerSPA(staticPath, indexPath string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // get the absolute path to prevent directory traversal
        path, err := filepath.Abs(r.URL.Path)
        if err != nil {
            // if we failed to get the absolute path respond with a 400 bad request
            // and stop
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        // prepend the path with the path to the static directory
        path = filepath.Join(staticPath, path)

        // check whether a file exists at the given path
        _, err = os.Stat(path)
        if os.IsNotExist(err) {
            // file does not exist, serve index.html
            http.ServeFile(w, r, filepath.Join(staticPath, indexPath))
            return
        } else if err != nil {
            // if we got an error (that wasn't that the file doesn't exist) stating the
            // file, return a 500 internal server error and stop
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        // otherwise, use http.FileServer to serve the static dir
        http.FileServer(http.Dir(staticPath)).ServeHTTP(w, r)
    }
}

