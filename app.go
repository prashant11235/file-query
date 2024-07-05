package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Promotions struct {
	Id             string
	Price          float64
	ExpirationDate string
}

type App struct {
	Router        *mux.Router
	promotionsMap map[string]Promotions
	mu            sync.RWMutex
	refFileName   string
}

func (a *App) Initialize() {
	a.Router = mux.NewRouter()
	a.initializeRoutes()
	a.loadFile()
}

func (a *App) Run(addr string) {
	log.Fatal(http.ListenAndServe(addr, a.Router))
}

func (a *App) loadFile() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	f, err := os.Open("data/promotions.csv")
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	a.promotionsMap = make(map[string]Promotions)

	for scanner.Scan() {
		line := scanner.Text()
		res := strings.Split(line, ",")
		id, priceStr, expirationDate := res[0], res[1], res[2]
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			return err
		}

		promotion, ok := a.promotionsMap[id]
		if !ok {
			promotion.Id = id
			promotion.Price = price
			promotion.ExpirationDate = expirationDate
		}

		a.promotionsMap[id] = promotion
	}
	return nil
}

func (a *App) getPromotions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid promotion ID")
		return
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	promotion, ok := a.promotionsMap[id.String()]

	if !ok {
		respondWithError(w, http.StatusNotFound, "Promotion not found")
	} else {
		respondWithJSON(w, http.StatusOK, promotion)
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (a *App) uploadPromotions(w http.ResponseWriter, r *http.Request) {
	// write new file
	file, handler, err := r.FormFile("file")
	fileName := r.FormValue("file_name")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	f, err := os.Create("data/" + handler.Filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, _ = io.WriteString(w, "File "+fileName+" Uploaded successfully")
	_, _ = io.Copy(f, file)

	// reload data
	a.loadFile()
}

func (a *App) initializeRoutes() {
	a.Router.HandleFunc("/promotions", a.uploadPromotions).Methods("POST")
	a.Router.HandleFunc("/promotions/{id}", a.getPromotions).Methods("GET")
}
