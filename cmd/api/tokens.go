package main

import (
	"errors"
	"github.com/danyelkeddah/go-greenlight/internal/data"
	"github.com/danyelkeddah/go-greenlight/internal/validator"
	"net/http"
	"time"
)

func (a *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := a.readJSON(w, r, &input)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)
	if !v.Valid() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := a.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.invalidCredentialsResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	match, err := user.Password.Matches(input.Password)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	if !match {
		a.invalidCredentialsResponse(w, r)
		return
	}

	token, err := a.models.Tokens.New(user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	err = a.writeJson(w, http.StatusCreated, envelop{"authentication_token": token}, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}
