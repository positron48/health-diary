package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchPlacesParsesOpenMeteo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":123,"name":"Липецк","admin1":"Липецкая область","country_code":"RU","timezone":"Europe/Moscow","latitude":52.6,"longitude":39.6}]}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, server.URL, server.URL, "open-meteo", time.Second, server.Client())
	places, err := client.SearchPlaces(context.Background(), "Липецк", 5)
	if err != nil || len(places) != 1 || places[0].Label != "Липецк" || places[0].ProviderPlaceID != "123" {
		t.Fatalf("unexpected places: %#v err=%v", places, err)
	}
}

func TestFetchDailyMarksCurrentIncomplete(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"daily":{"time":["` + today + `"],"temperature_2m_max":[10],"temperature_2m_min":[1],"temperature_2m_mean":[5],"precipitation_sum":[0],"weather_code":[1]},"hourly":{"time":["` + today + `T12:00"],"pressure_msl":[1010],"relative_humidity_2m":[50]}}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, server.URL, server.URL, "open-meteo", time.Second, server.Client())
	obs, err := client.FetchDaily(context.Background(), 52.6, 39.6, time.Now().UTC(), time.Now().UTC(), "UTC")
	if err != nil || len(obs) != 1 || obs[0].IsComplete {
		t.Fatalf("current day must be incomplete: %#v err=%v", obs, err)
	}
}
