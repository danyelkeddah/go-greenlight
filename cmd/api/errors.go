package main

import (
	"fmt"
	"net/http"
)

func (a *application) logError(r *http.Request, err error) {
	a.logger.PrintError(err, map[string]string{
		"request_method": r.Method,
		"request_url":    r.URL.String(),
	})
}

func (a *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	env := envelop{"error": message}
	err := a.writeJson(w, status, env, nil)
	if err != nil {
		a.logError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	a.logError(r, err)
	message := "the server encountered a problem and could not process your request"
	a.errorResponse(w, r, http.StatusInternalServerError, message)
}

func (a *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	a.errorResponse(w, r, http.StatusNotFound, message)
}

func (a *application) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	a.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

func (a *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	a.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (a *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	a.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

func (a *application) editConflictResponse(w http.ResponseWriter, r *http.Request) {
	message := "unable to update the record due to an edit conflict, please try again"
	a.errorResponse(w, r, http.StatusConflict, message)
}

func (a *application) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request) {
	message := "rate limit exceeded"
	a.errorResponse(w, r, http.StatusTooManyRequests, message)
}

func (a *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	message := "invalid authentication credentials"
	a.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (a *application) invalidAuthenticationTokenResponse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	message := "invalid or missing authentication token"
	a.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (a *application) authenticationRequiredResponse(w http.ResponseWriter, r *http.Request) {
	message := "you must be authenticated to access this resource"
	a.errorResponse(w, r, http.StatusUnauthorized, message)
}
func (a *application) inactiveAccountResponse(w http.ResponseWriter, r *http.Request) {
	message := "your user account must be activated to access this resource"
	a.errorResponse(w, r, http.StatusForbidden, message)
}
