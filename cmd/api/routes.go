package main

import (
	"expvar"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func (a *application) routes() *httprouter.Router {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(a.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(a.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", a.enableCORS(a.rateLimit(a.authenticate(a.healthcheckHandler))))

	router.HandlerFunc(http.MethodGet, "/v1/movies", a.enableCORS(a.rateLimit(a.authenticate(a.requireActivatedUser(a.listMoviesHandler)))))
	router.HandlerFunc(http.MethodPost, "/v1/movies", a.enableCORS(a.rateLimit(a.authenticate(a.requireActivatedUser(a.createMovieHandler)))))
	router.HandlerFunc(http.MethodGet, "/v1/movies/:id", a.enableCORS(a.rateLimit(a.authenticate(a.requireActivatedUser(a.showMovieHandler)))))
	router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", a.enableCORS(a.rateLimit(a.authenticate(a.requireActivatedUser(a.updateMovieHandler)))))
	router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", a.enableCORS(a.rateLimit(a.authenticate(a.requireActivatedUser(a.deleteMovieHandler)))))
	router.HandlerFunc(http.MethodPost, "/v1/users", a.enableCORS(a.authenticate(a.registerUserHandler)))
	router.HandlerFunc(http.MethodPut, "/v1/users/activated", a.enableCORS(a.authenticate(a.activateUserHandler)))
	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", a.enableCORS(a.authenticate(a.createAuthenticationTokenHandler)))

	router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())

	router.PanicHandler = a.recoverPanic

	return router
}
