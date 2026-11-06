// Package httpresponse defines the JSON response contract for all HTTP adapters.
package httpresponse

import (
	"encoding/json"
	"net/http"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SuccessBody struct {
	Data any `json:"data"`
}

type ErrorBody struct {
	Error Error `json:"error"`
}

func Success(w http.ResponseWriter, status int, data any) {

	write(w, status, SuccessBody{
		Data: data,
	})
}

func Failure(w http.ResponseWriter, status int, code, message string) {
	write(w, status, ErrorBody{
		Error: Error{
			Code:    code,
			Message: message,
		},
	})
}

func NoContent(w http.ResponseWriter) {

	w.WriteHeader(http.StatusNoContent)
}

func write(w http.ResponseWriter, status int, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		encoded = []byte(`{"error":{"code":"internal_error","message":"internal server error"}}`)
	}
	// Encode before committing headers so an encoding failure is still a valid JSON response.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(encoded, '\n'))
}
