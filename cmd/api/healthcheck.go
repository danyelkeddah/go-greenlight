package main

import (
	"net/http"
)

func (a *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	env := envelop{
		"status": "available",
		"system_info": map[string]string{
			"environment": a.config.env,
			"version":     version,
		},
	}

	err := a.writeJson(w, http.StatusOK, env, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}
