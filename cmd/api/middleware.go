package main

import (
	"errors"
	"fmt"
	"github.com/danyelkeddah/go-greenlight/internal/data"
	"github.com/danyelkeddah/go-greenlight/internal/validator"
	"github.com/tomasen/realip"
	"golang.org/x/time/rate"
	"net/http"
	"strings"
	"sync"
	"time"
)

func (a *application) recoverPanic(w http.ResponseWriter, r *http.Request, err interface{}) {
	w.Header().Set("Connection", "close")
	a.serverErrorResponse(w, r, fmt.Errorf("%s", err))
}

func (a *application) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	// Define a client struct to hold the rate limiter and last seen time for each client.
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	var (
		mu sync.Mutex
		// Update the map so the values are pointers to a client struct.
		clients = make(map[string]*client)
	)

	// Launch a background goroutine which removes old entries from the clients map once every minute.
	go func() {
		// Lock the mutex to prevent any rate limiter checks from happening while the cleanup is taking place.
		mu.Lock()
		// Loop through all clients. If they haven't been seen within the last three minutes, delete the corresponding entry from the map
		for ip, client := range clients {
			if time.Since(client.lastSeen) > 3*time.Minute {
				delete(clients, ip)
			}
		}
		// mportantly, unlock the mutex when the cleanup is complete.
		mu.Unlock()
	}()

	return func(w http.ResponseWriter, r *http.Request) {
		if a.config.limiter.enabled {
			ip := realip.FromRequest(r)

			mu.Lock()

			if _, found := clients[ip]; !found {
				// Create and add a new client struct to the map if it doesn't already exist.
				clients[ip] = &client{limiter: rate.NewLimiter(rate.Limit(a.config.limiter.rps), a.config.limiter.burst)}
			}

			// Update the last seen time for the client.
			clients[ip].lastSeen = time.Now()

			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				a.rateLimitExceededResponse(w, r)
				return
			}

			mu.Unlock()
		}
		next.ServeHTTP(w, r)
	}
}

func (a *application) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// indicates any cache that the response may vary based on the value of the authorization header in the request.
		w.Header().Add("Vary", "Authorization")

		authorizationHeader := r.Header.Get("Authorization")

		if authorizationHeader == "" {
			r = a.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			a.invalidAuthenticationTokenResponse(w, r)
			return
		}

		token := headerParts[1]

		v := validator.New()

		if data.ValidateTokenPlainText(v, token); !v.Valid() {
			a.invalidAuthenticationTokenResponse(w, r)
			return
		}

		user, err := a.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				a.invalidAuthenticationTokenResponse(w, r)
			default:
				a.serverErrorResponse(w, r, err)
			}
			return
		}

		r = a.contextSetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

func (a *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		user := a.contextGetUser(r)

		if user.IsAnonymous() {
			a.authenticationRequiredResponse(w, r)
			return
		}

		if !user.Activated {
			a.inactiveAccountResponse(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *application) enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")

		origin := r.Header.Get("Origin")
		if origin != "" {
			// check if the request origin exactly matches one of them.
			// If there are no trusted origins, then the loop won't be iterated.
			for i := range a.config.cors.trustedOrigins {
				if origin == a.config.cors.trustedOrigins[i] {
					// if match, them set Access-Control-Allow-Origin as origin value and break out of the loop
					w.Header().Set("Access-Control-Allow-Origin", origin)
					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						w.Header().Set("Access-Control-Request-Method", "OPTIONS, PUT, PATCH, DELETE")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

						w.WriteHeader(http.StatusOK)
						return
					}
					break
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}

//func (a *application) metrics(next http.HandlerFunc) http.HandlerFunc {
//	totalRequestsReceived := expvar.NewInt("total_requests_received")
//	totalResponsesSent := expvar.NewInt("total_responses_sent")
//	totalProcessingTimeMicrosecond := expvar.NewInt("total_processing_time_μs")
//
//	return func(w http.ResponseWriter, r *http.Request) {
//		start := time.Now()
//		totalRequestsReceived.Add(1)
//		next.ServeHTTP(w, r)
//		totalResponsesSent.Add(1)
//		duration := time.Since(start).Milliseconds()
//		totalProcessingTimeMicrosecond.Add(duration)
//	}
//
//}
