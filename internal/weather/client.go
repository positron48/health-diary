package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	http         *http.Client
	archiveURL   string
	forecastURL  string
	geocodingURL string
	provider     string
}

type PlaceCandidate struct {
	ProviderPlaceID string  `json:"provider_place_id"`
	Label           string  `json:"label"`
	Region          string  `json:"region,omitempty"`
	CountryCode     string  `json:"country_code"`
	Timezone        string  `json:"timezone"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
}

type DailyObservation struct {
	LocalDate       time.Time
	TempMinC        *float64
	TempMaxC        *float64
	TempMeanC       *float64
	PressureMeanHPa *float64
	HumidityMeanPct *float64
	PrecipitationMM *float64
	WeatherCode     *int
	IsComplete      bool
}

func NewClient(archiveURL, forecastURL, geocodingURL, provider string, timeout time.Duration, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	if provider == "" {
		provider = "open-meteo"
	}
	return &Client{
		http:         httpClient,
		archiveURL:   strings.TrimRight(archiveURL, "/"),
		forecastURL:  strings.TrimRight(forecastURL, "/"),
		geocodingURL: strings.TrimRight(geocodingURL, "/"),
		provider:     provider,
	}
}

func (c *Client) Provider() string { return c.provider }

func (c *Client) SearchPlaces(ctx context.Context, query string, limit int) ([]PlaceCandidate, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	endpoint := fmt.Sprintf("%s/v1/search?name=%s&count=%d&language=ru&format=json",
		c.geocodingURL, url.QueryEscape(query), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("geocoding status %d", resp.StatusCode)
	}
	var parsed struct {
		Results []struct {
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			Admin1      string  `json:"admin1"`
			CountryCode string  `json:"country_code"`
			Timezone    string  `json:"timezone"`
			Latitude    float64 `json:"latitude"`
			Longitude   float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]PlaceCandidate, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		tz := item.Timezone
		if tz == "" {
			tz = "Europe/Moscow"
		}
		out = append(out, PlaceCandidate{
			ProviderPlaceID: strconv.Itoa(item.ID),
			Label:           item.Name,
			Region:          item.Admin1,
			CountryCode:     item.CountryCode,
			Timezone:        tz,
			Latitude:        item.Latitude,
			Longitude:       item.Longitude,
		})
	}
	return out, nil
}

func (c *Client) FetchDaily(ctx context.Context, lat, lon float64, from, to time.Time, timezone string) ([]DailyObservation, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	start := from.Format("2006-01-02")
	end := to.Format("2006-01-02")
	today := time.Now().In(mustLocation(timezone)).Format("2006-01-02")
	useForecast := end >= today
	base := c.archiveURL
	path := "/v1/archive"
	if useForecast {
		base = c.forecastURL
		path = "/v1/forecast"
	}
	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%.6f", lat))
	params.Set("longitude", fmt.Sprintf("%.6f", lon))
	params.Set("start_date", start)
	params.Set("end_date", end)
	params.Set("timezone", timezone)
	params.Set("daily", "temperature_2m_max,temperature_2m_min,temperature_2m_mean,precipitation_sum,weather_code")
	params.Set("hourly", "pressure_msl,relative_humidity_2m")
	endpoint := fmt.Sprintf("%s%s?%s", base, path, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("weather status %d", resp.StatusCode)
	}
	var parsed struct {
		Daily struct {
			Time          []string   `json:"time"`
			TempMax       []*float64 `json:"temperature_2m_max"`
			TempMin       []*float64 `json:"temperature_2m_min"`
			TempMean      []*float64 `json:"temperature_2m_mean"`
			Precipitation []*float64 `json:"precipitation_sum"`
			WeatherCode   []*int     `json:"weather_code"`
		} `json:"daily"`
		Hourly struct {
			Time     []string   `json:"time"`
			Pressure []*float64 `json:"pressure_msl"`
			Humidity []*float64 `json:"relative_humidity_2m"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	pressureByDay := averageByLocalDate(parsed.Hourly.Time, parsed.Hourly.Pressure)
	humidityByDay := averageByLocalDate(parsed.Hourly.Time, parsed.Hourly.Humidity)
	out := make([]DailyObservation, 0, len(parsed.Daily.Time))
	for i, day := range parsed.Daily.Time {
		localDate, err := time.ParseInLocation("2006-01-02", day, mustLocation(timezone))
		if err != nil {
			continue
		}
		obs := DailyObservation{LocalDate: localDate, IsComplete: day < today}
		if i < len(parsed.Daily.TempMin) {
			obs.TempMinC = parsed.Daily.TempMin[i]
		}
		if i < len(parsed.Daily.TempMax) {
			obs.TempMaxC = parsed.Daily.TempMax[i]
		}
		if i < len(parsed.Daily.TempMean) {
			obs.TempMeanC = parsed.Daily.TempMean[i]
		}
		if i < len(parsed.Daily.Precipitation) {
			obs.PrecipitationMM = parsed.Daily.Precipitation[i]
		}
		if i < len(parsed.Daily.WeatherCode) {
			obs.WeatherCode = parsed.Daily.WeatherCode[i]
		}
		if value, ok := pressureByDay[day]; ok {
			copy := value
			obs.PressureMeanHPa = &copy
		}
		if value, ok := humidityByDay[day]; ok {
			copy := value
			obs.HumidityMeanPct = &copy
		}
		out = append(out, obs)
	}
	return out, nil
}

func averageByLocalDate(times []string, values []*float64) map[string]float64 {
	sums := map[string]float64{}
	counts := map[string]int{}
	for i, raw := range times {
		if i >= len(values) || values[i] == nil {
			continue
		}
		day := raw
		if len(raw) >= 10 {
			day = raw[:10]
		}
		sums[day] += *values[i]
		counts[day]++
	}
	out := map[string]float64{}
	for day, sum := range sums {
		if counts[day] == 0 {
			continue
		}
		out[day] = sum / float64(counts[day])
	}
	return out
}

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}
