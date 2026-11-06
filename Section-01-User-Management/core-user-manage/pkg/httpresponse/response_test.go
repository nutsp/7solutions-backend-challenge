package httpresponse

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestResponseContract(t *testing.T) {
	tests := []struct {
		name   string
		write  func(*httptest.ResponseRecorder)
		status int
		body   string
	}{
		{
			"success",
			func(w *httptest.ResponseRecorder) {

				Success(w, 200, map[string]string{"id": "user"})
			},
			200,
			`{"data":{"id":"user"}}`,
		},
		{"null data", func(w *httptest.ResponseRecorder) {

			Success(w, 200, nil)
		}, 200, `{"data":null}`},
		{"empty array", func(w *httptest.ResponseRecorder) {

			Success(w, 200, []string{})
		}, 200, `{"data":[]}`},
		{
			"safe failure",
			func(w *httptest.ResponseRecorder) {

				Failure(w, 400, "invalid_request", "invalid request")
			},
			400,
			`{"error":{"code":"invalid_request","message":"invalid request"}}`,
		},
		{
			"unencodable data",
			func(w *httptest.ResponseRecorder) {

				Success(w, 200, make(chan int))
			},
			500,
			`{"error":{"code":"internal_error","message":"internal server error"}}`,
		},
		{"no content", func(w *httptest.ResponseRecorder) {

			NoContent(w)
		}, 204, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.write(w)
			if w.Code != tt.status {
				t.Fatalf("status=%d; want %d", w.Code, tt.status)
			}
			if tt.status == 204 {
				if w.Body.Len() != 0 {
					t.Fatal("204 must not have a body")
				}
				return
			}
			if w.Header().Get("Content-Type") != "application/json" {
				t.Fatal("missing JSON content type")
			}
			var actual, expected any
			if err := json.Unmarshal(w.Body.Bytes(), &actual); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tt.body), &expected); err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(actual)
			want, _ := json.Marshal(expected)
			if string(got) != string(want) {
				t.Fatalf("got %s; want %s", got, want)
			}
		})
	}
}
