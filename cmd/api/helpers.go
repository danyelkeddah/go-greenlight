package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/danyelkeddah/go-greenlight/internal/validator"
	"github.com/julienschmidt/httprouter"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type envelop map[string]any

func (a *application) readIdParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

func (a *application) writeJson(w http.ResponseWriter, status int, data any, headers http.Header) error {
	j, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	j = append(j, '\n')
	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(j)
	return nil
}

func (a *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// Decode the request into the target destination
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // remove unwanted fields

	err := dec.Decode(dst)
	if err != nil {
		// If there is an error during decoding, start the triage...
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		/**
		Use errors.As() to check weather the error has the type *json.SyntaxError.
		If it does, then return a plain-english error message which includes the location of the problem.
		*/
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		/**
		In some circumstances Decode() may also return an io.ErrUnexpectedEOF error for syntax errors in the JSON.return generic error message
		*/
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		/**
		Likewise, catch any *json.UnmarshalTypeError errors,
		These occur when the JSON value is the wrong type for the target destination.
		If the error relates to a specific field, then we include that in our error message to make it easier for the client to debug.
		*/
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		/**
		An io.EOF error will be returned by Decode() if the request body is empty.
		We check for this with errors.Is() and return a plain-english error message instead.
		*/
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		// prevent unknown fields
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldsName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldsName)

		// prevent big size objects
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)

		/**
		A json.InvalidUnmarshalError error will be returned if we pass something
		that is not a non-nil pointer to Decode(). We catch this and panic,
		rather than returning an error to our handler. At the end of this chapter
		we'll talk about panicking versus returning errors, and discuss why it's an
		appropriate thing to do in this specific situation.
		*/
		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		// For anything else, return the error message as-is.
		default:
			return err
		}
	}
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

func (a *application) readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}

	return s
}

func (a *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)
	if csv == "" {
		return defaultValue
	}

	return strings.Split(csv, ",")
}

func (a *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}

	return i
}

func (a *application) background(fn func()) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() {
			if err := recover(); err != nil {
				a.logger.PrintError(fmt.Errorf("%s", err), nil)
			}
		}()
		fn()
	}()
}
