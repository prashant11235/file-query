package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	main "test.com/storage"
)

var a main.App

func TestMain(m *testing.M) {
	a.Initialize()

	code := m.Run()
	os.Exit(code)
}

func TestGetNonExistentPromotion(t *testing.T) {
	req, _ := http.NewRequest("GET", "/promotions/5e0fca24-1111-1111-84a3-7f0813719d19", nil)
	response := executeRequest(req)

	checkResponseCode(t, http.StatusNotFound, response.Code)

	var m map[string]string
	json.Unmarshal(response.Body.Bytes(), &m)
	if m["error"] != "Promotion not found" {
		t.Errorf("Expected 'error' key of response to be set to 'Promotion not found'. Got '%s'", m["error"])
	}
}

func TestGetPromotion(t *testing.T) {
	req, _ := http.NewRequest("GET", "/promotions/d018ef0b-dbd9-48f1-ac1a-eb4d90e57118", nil)
	response := executeRequest(req)
	checkResponseCode(t, http.StatusOK, response.Code)
}

func executeRequest(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	a.Router.ServeHTTP(rr, req)

	return rr
}

func checkResponseCode(t *testing.T, expected, actual int) {
	if expected != actual {
		t.Errorf("Expected response code %d. Got %d\n", expected, actual)
	}
}
