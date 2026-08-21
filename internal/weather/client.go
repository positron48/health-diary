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

type HourlyObservation struct {
	ObservedAt      time.Time
	LocalDate       time.Time
	TemperatureC    *float64
	PressureMSLHPa  *float64
	HumidityPct     *float64
	PrecipitationMM *float64
	IsComplete      bool
}

type FetchResult struct {
	Daily  []DailyObservation
	Hourly []HourlyObservation
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

func (c *Client) FetchDaily(ctx context.Context, lat, lon float64, from, to time.Time, timezone string) (FetchResult, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	loc := mustLocation(timezone)
	today := time.Now().In(loc)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	startDate := dateInLocation(from, loc)
	endDate := dateInLocation(to, loc)
	if endDate.Before(startDate) {
		return FetchResult{}, fmt.Errorf("weather range end before start")
	}
	if endDate.Before(todayDate) {
		return c.fetchRange(ctx, c.archiveURL, "/v1/archive", lat, lon, startDate, endDate, loc, today)
	}
	if !startDate.Before(todayDate) {
		return c.fetchRange(ctx, c.forecastURL, "/v1/forecast", lat, lon, startDate, endDate, loc, today)
	}
	archive, err := c.fetchRange(ctx, c.archiveURL, "/v1/archive", lat, lon, startDate, todayDate.AddDate(0, 0, -1), loc, today)
	if err != nil {
		return FetchResult{}, err
	}
	forecast, err := c.fetchRange(ctx, c.forecastURL, "/v1/forecast", lat, lon, todayDate, endDate, loc, today)
	if err != nil {
		return FetchResult{}, err
	}
	archive.Daily = append(archive.Daily, forecast.Daily...)
	archive.Hourly = append(archive.Hourly, forecast.Hourly...)
	return archive, nil
}

func (c *Client) fetchRange(ctx context.Context, base, path string, lat, lon float64, from, to time.Time, loc *time.Location, now time.Time) (FetchResult, error) {
	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%.6f", lat))
	params.Set("longitude", fmt.Sprintf("%.6f", lon))
	params.Set("start_date", from.Format("2006-01-02"))
	params.Set("end_date", to.Format("2006-01-02"))
	params.Set("timezone", loc.String())
	params.Set("daily", "temperature_2m_max,temperature_2m_min,temperature_2m_mean,precipitation_sum,weather_code")
	params.Set("hourly", "temperature_2m,pressure_msl,relative_humidity_2m,precipitation")
	endpoint := fmt.Sprintf("%s%s?%s", base, path, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return FetchResult{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return FetchResult{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return FetchResult{}, err
	}
	if resp.StatusCode >= 400 {
		return FetchResult{}, fmt.Errorf("weather status %d", resp.StatusCode)
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
			Time          []string   `json:"time"`
			Temperature   []*float64 `json:"temperature_2m"`
			Pressure      []*float64 `json:"pressure_msl"`
			Humidity      []*float64 `json:"relative_humidity_2m"`
			Precipitation []*float64 `json:"precipitation"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return FetchResult{}, err
	}
	pressureByDay := averageByLocalDate(parsed.Hourly.Time, parsed.Hourly.Pressure)
	humidityByDay := averageByLocalDate(parsed.Hourly.Time, parsed.Hourly.Humidity)
	today := now.Format("2006-01-02")
	result := FetchResult{
		Daily:  make([]DailyObservation, 0, len(parsed.Daily.Time)),
		Hourly: make([]HourlyObservation, 0, len(parsed.Hourly.Time)),
	}
	for i, day := range parsed.Daily.Time {
		localDate, err := time.ParseInLocation("2006-01-02", day, loc)
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
		result.Daily = append(result.Daily, obs)
	}
	for i, raw := range parsed.Hourly.Time {
		observedAt, err := time.ParseInLocation("2006-01-02T15:04", raw, loc)
		if err != nil {
			continue
		}
		item := HourlyObservation{
			ObservedAt: observedAt,
			LocalDate:  time.Date(observedAt.Year(), observedAt.Month(), observedAt.Day(), 0, 0, 0, 0, loc),
			IsComplete: !observedAt.Add(time.Hour).After(now),
		}
		if i < len(parsed.Hourly.Temperature) {
			item.TemperatureC = parsed.Hourly.Temperature[i]
		}
		if i < len(parsed.Hourly.Pressure) {
			item.PressureMSLHPa = parsed.Hourly.Pressure[i]
		}
		if i < len(parsed.Hourly.Humidity) {
			item.HumidityPct = parsed.Hourly.Humidity[i]
		}
		if i < len(parsed.Hourly.Precipitation) {
			item.PrecipitationMM = parsed.Hourly.Precipitation[i]
		}
		result.Hourly = append(result.Hourly, item)
	}
	return result, nil
}

func dateInLocation(value time.Time, loc *time.Location) time.Time {
	date := value.Format("2006-01-02")
	parsed, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return value.In(loc)
	}
	return parsed
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
