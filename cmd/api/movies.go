package main

import (
	"errors"
	"fmt"
	"github.com/danyelkeddah/go-greenlight/internal/data"
	"github.com/danyelkeddah/go-greenlight/internal/validator"
	"net/http"
)

func (a *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		RunTime data.Runtime `json:"run_time"`
		Genres  []string     `json:"genres"`
	}

	err := a.readJSON(w, r, &input)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}
	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		RunTime: input.RunTime,
		Genres:  input.Genres,
	}
	v := validator.New()

	if data.ValidateMovie(v, movie); !v.Valid() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.models.Movies.Insert(movie)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))

	err = a.writeJson(w, http.StatusCreated, envelop{"movie": movie}, headers)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIdParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}
	movie, err := a.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}
	err = a.writeJson(w, http.StatusOK, envelop{"movie": movie}, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIdParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
	}
	movie, err := a.models.Movies.Get(id)

	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"run_time"`
		Genres  []string      `json:"genres"`
	}

	err = a.readJSON(w, r, &input)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}
	if input.Year != nil {
		movie.Year = *input.Year
	}

	if input.Runtime != nil {
		movie.RunTime = *input.Runtime
	}

	if input.Genres != nil {
		movie.Genres = input.Genres
	}

	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.models.Movies.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			a.editConflictResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	err = a.writeJson(w, http.StatusOK, envelop{"movie": movie}, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}

}

func (a *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIdParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	err = a.models.Movies.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	err = a.writeJson(w, http.StatusOK, envelop{"message": "movie successfully deleted"}, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title  string
		Genres []string
		data.Filters
	}

	v := validator.New()
	qs := r.URL.Query()

	input.Title = a.readString(qs, "title", "")
	input.Genres = a.readCSV(qs, "genres", []string{})
	input.Filters.Page = a.readInt(qs, "page", 1, v)
	input.Filters.PageSize = a.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = a.readString(qs, "sort", "id")
	input.Filters.SortSafeList = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	movies, metadata, err := a.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	err = a.writeJson(w, http.StatusOK, envelop{"movies": movies, "metadata": metadata}, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}
