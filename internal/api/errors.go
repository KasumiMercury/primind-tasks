package api

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

const (
	StatusAlreadyExists   = "ALREADY_EXISTS"
	StatusInvalidArgument = "INVALID_ARGUMENT"
	StatusInternal        = "INTERNAL"
)

func WriteError(w http.ResponseWriter, code int, status string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Status:  status,
		},
	})
}
