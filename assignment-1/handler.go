package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	version          = "v1"
	countriesBaseURL = "http://129.241.150.113:8080/v3.1"
	currencyBaseURL  = "http://129.241.150.113:9090/currency"
)

var (
	startTime  time.Time
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

type errResp struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errResp{Error: msg})
}

func uptimeSeconds() int64 {
	return int64(time.Since(startTime).Seconds())
}

func normalizeISO2(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func validISO2(code string) bool {
	if len(code) != 2 {
		return false
	}
	for _, ch := range code {
		if ch < 'a' || ch > 'z' {
			return false
		}
	}
	return true
}

/* -------------------- STATUS endpoint -------------------- */

type statusResponse struct {
	RestCountriesAPI any    `json:"restcountriesapi"`
	CurrenciesAPI    any    `json:"currenciesapi"`
	Version          string `json:"version"`
	Uptime           int64  `json:"uptime"`
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Use lightweight “known-good” probes
	restStatus := probeHTTP(fmt.Sprintf("%s/alpha/no", countriesBaseURL))
	currStatus := probeHTTP(fmt.Sprintf("%s/NOK", currencyBaseURL))

	// Spec: 200 if everything OK, appropriate error otherwise.
	overall := http.StatusOK
	if restStatus != http.StatusOK || currStatus != http.StatusOK {
		overall = http.StatusBadGateway
	}

	resp := statusResponse{
		RestCountriesAPI: restStatus,
		CurrenciesAPI:    currStatus,
		Version:          version,
		Uptime:           uptimeSeconds(),
	}
	writeJSON(w, overall, resp)
}

func probeHTTP(url string) int {
	resp, err := httpClient.Get(url)
	if err != nil {
		return http.StatusBadGateway
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

/* -------------------- COUNTRIES models -------------------- */

// Minimal fields needed for info/exchange
type countriesName struct {
	Common string `json:"common"`
}

type countriesFlags struct {
	PNG string `json:"png"`
	SVG string `json:"svg"`
}

type countriesCountry struct {
	Name       countriesName              `json:"name"`
	Continents []string                   `json:"continents"`
	Population int64                      `json:"population"`
	Area       float64                    `json:"area"`
	Languages  map[string]string          `json:"languages"`
	Borders    []string                   `json:"borders"`
	Flags      countriesFlags             `json:"flags"`
	Capital    []string                   `json:"capital"`
	Currencies map[string]json.RawMessage `json:"currencies"` // keys are currency codes
}

// /alpha/{code} can return an object or an array; support both
func fetchCountryAlpha(code string) (*countriesCountry, int, error) {
	url := fmt.Sprintf("%s/alpha/%s", countriesBaseURL, code)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, 0, err
	}

	// Try array
	var arr []countriesCountry
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return &arr[0], http.StatusOK, nil
	}

	return nil, 0, fmt.Errorf("unexpected alpha response shape")
}

func firstCurrencyCodeSorted(m map[string]json.RawMessage) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys[0]
}

/* -------------------- INFO endpoint -------------------- */

type infoResponse struct {
	Name       string            `json:"name"`
	Continents []string          `json:"continents"`
	Population int64             `json:"population"`
	Area       float64           `json:"area"`
	Languages  map[string]string `json:"languages"`
	Borders    []string          `json:"borders"`
	Flag       string            `json:"flag"`
	Capital    string            `json:"capital"`
}

func InfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	code := strings.TrimPrefix(r.URL.Path, "/countryinfo/v1/info/")
	code = normalizeISO2(code)

	if !validISO2(code) {
		writeJSONError(w, http.StatusBadRequest, "two_letter_country_code must be 2 letters (ISO 3166-2), e.g. /countryinfo/v1/info/no")
		return
	}

	c, st, err := fetchCountryAlpha(code)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "failed to call countries service")
		return
	}
	if st == http.StatusNotFound || c == nil {
		writeJSONError(w, http.StatusNotFound, "country not found")
		return
	}
	if st != http.StatusOK {
		writeJSONError(w, http.StatusBadGateway, "countries service returned non-200")
		return
	}

	capital := ""
	if len(c.Capital) > 0 {
		capital = c.Capital[0]
	}

	flag := c.Flags.PNG
	if flag == "" {
		flag = c.Flags.SVG
	}

	out := infoResponse{
		Name:       c.Name.Common,
		Continents: c.Continents,
		Population: c.Population,
		Area:       c.Area,
		Languages:  c.Languages,
		Borders:    c.Borders,
		Flag:       flag,
		Capital:    capital,
	}

	writeJSON(w, http.StatusOK, out)
}

/* -------------------- CURRENCY models -------------------- */

type upstreamCurrencyResponse struct {
	Result string             `json:"result"`
	Rates  map[string]float64 `json:"rates"`
}

func fetchRates(base string) (*upstreamCurrencyResponse, int, error) {
	url := fmt.Sprintf("%s/%s", currencyBaseURL, base)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	var out upstreamCurrencyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, 0, err
	}
	return &out, http.StatusOK, nil
}

/* -------------------- EXCHANGE endpoint -------------------- */

type exchangeResponse struct {
	Country       string             `json:"country"`
	BaseCurrency  string             `json:"base-currency"`
	ExchangeRates map[string]float64 `json:"exchange-rates"`
}

func ExchangeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	code := strings.TrimPrefix(r.URL.Path, "/countryinfo/v1/exchange/")
	code = normalizeISO2(code)

	if !validISO2(code) {
		writeJSONError(w, http.StatusBadRequest, "two_letter_country_code must be 2 letters (ISO 3166-2), e.g. /countryinfo/v1/exchange/no")
		return
	}

	// 1) Fetch input country
	input, st, err := fetchCountryAlpha(code)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "failed to call countries service")
		return
	}
	if st == http.StatusNotFound || input == nil {
		writeJSONError(w, http.StatusNotFound, "country not found")
		return
	}
	if st != http.StatusOK {
		writeJSONError(w, http.StatusBadGateway, "countries service returned non-200")
		return
	}

	// 2) Determine base currency (first currency key)
	base := strings.ToUpper(firstCurrencyCodeSorted(input.Currencies))
	if base == "" || len(base) != 3 {
		writeJSONError(w, http.StatusBadGateway, "input country has no valid currency")
		return
	}

	// 3) Collect neighbour currencies
	neighCurrencies := make(map[string]struct{})
	for _, cca3 := range input.Borders {
		cca3 = strings.TrimSpace(cca3)
		if cca3 == "" {
			continue
		}

		nc, st2, err := fetchCountryAlpha(cca3) // alpha accepts cca3 too in most implementations
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "failed to call countries service for neighbours")
			return
		}
		if st2 != http.StatusOK || nc == nil {
			writeJSONError(w, http.StatusBadGateway, "countries service failed neighbour lookup")
			return
		}

		ccy := strings.ToUpper(firstCurrencyCodeSorted(nc.Currencies))
		if ccy == "" || len(ccy) != 3 {
			continue
		}
		if ccy == base {
			continue
		}
		neighCurrencies[ccy] = struct{}{}
	}

	// If no neighbours: return empty map (still 200)
	if len(neighCurrencies) == 0 {
		out := exchangeResponse{
			Country:       input.Name.Common,
			BaseCurrency:  base,
			ExchangeRates: map[string]float64{},
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	// 4) Fetch rates once
	ratesResp, st3, err := fetchRates(base)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "failed to call currency service")
		return
	}
	if st3 != http.StatusOK || ratesResp == nil {
		writeJSONError(w, http.StatusBadGateway, "currency service returned non-200")
		return
	}
	if ratesResp.Result != "" && ratesResp.Result != "success" {
		writeJSONError(w, http.StatusBadGateway, "currency service returned result != success")
		return
	}

	// 5) Filter rates to neighbour currencies
	outRates := make(map[string]float64)
	for ccy := range neighCurrencies {
		if v, ok := ratesResp.Rates[ccy]; ok {
			outRates[ccy] = v
		}
	}

	out := exchangeResponse{
		Country:       input.Name.Common,
		BaseCurrency:  base,
		ExchangeRates: outRates,
	}
	writeJSON(w, http.StatusOK, out)
}
