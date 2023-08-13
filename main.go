package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/html"
)

type Pharmacy struct {
	Name    string `json:"name"`
	InStock bool   `json:"in_stock"`
}

func getAvailability(w http.ResponseWriter, r *http.Request) {
	lat := r.URL.Query().Get("lat")
	lng := r.URL.Query().Get("lng")
	DVnr := r.URL.Query().Get("DVnr")

	if lat == "" || lng == "" || DVnr == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	url, err := url.Parse("https://laakehakupalvelu.apteekkariliitto.fi/EtsiApteekkiSaatavuus.aspx")
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse URL")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	q := url.Query()
	q.Set("lat", lat)
	q.Set("lng", lng)
	q.Set("DVnr", DVnr)
	url.RawQuery = q.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		log.Error().Err(err).Msg("Failed to send HTTP request")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse HTML")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pharmacyNodes, err := htmlquery.QueryAll(doc, `//div[@class="otsake"]`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query pharmacy nodes")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pharmacies := make([]Pharmacy, 0, len(pharmacyNodes))
	for _, pharmacyNode := range pharmacyNodes {
		var p Pharmacy
		nameNode, _ := htmlquery.Query(pharmacyNode, `//div[@class="nimi"]`)
		if nameNode != nil && nameNode.FirstChild != nil {
			p.Name = nameNode.FirstChild.Data
		}

		inStockNode, _ := htmlquery.Query(pharmacyNode, `//div[contains(@class, 'varastossa')]`)
		if inStockNode != nil && inStockNode.FirstChild != nil {
			p.InStock = inStockNode.FirstChild.Data == "Kyllä"
		}

		pharmacies = append(pharmacies, p)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(pharmacies)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/availability", getAvailability)

	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info().Msg("Server started at :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Panic().Err(err).Msg("Server Shutdown Failed")
	}
	log.Info().Msg("Server gracefully stopped")
}
