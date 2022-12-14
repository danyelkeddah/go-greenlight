package main

import (
	"errors"
	"github.com/danyelkeddah/go-greenlight/internal/data"
	"github.com/danyelkeddah/go-greenlight/internal/validator"
	"net/http"
	"time"
)

func (a *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := a.readJSON(w, r, &input)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()
	if data.ValidateUser(v, user); !v.Valid() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.models.Users.Insert(user)

	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			a.failedValidationResponse(w, r, v.Errors)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	token, err := a.models.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	a.background(func() {
		data := map[string]any{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}
		err = a.mailer.Send(user.Email, "user_welcome.go.html", data)
		if err != nil {
			a.logger.PrintError(err, nil)
		}
	})

	err = a.writeJson(w, http.StatusCreated, envelop{"user": user}, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlainText string `json:"token"`
	}

	err := a.readJSON(w, r, &input)
	if err != nil {
		a.badRequestResponse(w, r, err)
	}
	v := validator.New()
	if data.ValidateTokenPlainText(v, input.TokenPlainText); !v.Valid() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}
	user, err := a.models.Users.GetForToken(data.ScopeActivation, input.TokenPlainText)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			a.failedValidationResponse(w, r, v.Errors)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}
	user.Activated = true
	err = a.models.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			a.editConflictResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	err = a.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
	// Send the updated user details to the client in a JSON response.
	err = a.writeJson(w, http.StatusOK, envelop{"user": user}, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}
