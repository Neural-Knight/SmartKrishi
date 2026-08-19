package tools

import (
	"context"
	"net/url"
	"strconv"
)

const dataGovBaseURL = "https://api.data.gov.in/resource/"

// Market returns crop price info from the data.gov.in Agmarknet resource. It
// ports Python market_api(crop, region="national").
//
// IMPORTANT (bug fix vs Python): this tool takes the CROP as its primary
// argument. The Python main_agent wrongly passed plan.location to every tool,
// so market_api received a location string where it expected a commodity. The
// Go executor routes plan.Crop here instead. See MIGRATION.md.
//
// region defaults to "national" when empty; any other value is sent as a state
// filter. On missing credentials or error it returns a fallback payload.
func (r *Registry) Market(ctx context.Context, crop, region string) map[string]any {
	if region == "" {
		region = "national"
	}
	fallback := map[string]any{"crop": crop, "latest": nil, "history": []float64{}, "source": "fallback"}

	if r.cfg.DataGovKey == "" || r.cfg.AgmarknetID == "" {
		return fallback
	}

	base := r.cfg.MarketBaseURL
	if base == "" {
		base = dataGovBaseURL + r.cfg.AgmarknetID
	}
	q := url.Values{}
	q.Set("api-key", r.cfg.DataGovKey)
	q.Set("format", "json")
	q.Set("limit", "20")
	q.Set("filters[commodity]", crop)
	if region != "national" {
		q.Set("filters[state]", region)
	}
	full := base + "?" + q.Encode()

	var data marketResponse
	if err := r.getJSON(ctx, full, &data); err != nil {
		return fallback
	}

	prices := make([]float64, 0, len(data.Records))
	for _, rec := range data.Records {
		if rec.ModalPrice == "" {
			continue
		}
		p, err := strconv.ParseFloat(rec.ModalPrice, 64)
		if err != nil {
			continue
		}
		prices = append(prices, p)
	}

	var latest any
	if len(prices) > 0 {
		latest = prices[len(prices)-1]
	}
	history := prices
	if len(history) > 10 {
		history = history[len(history)-10:]
	}

	return map[string]any{
		"crop":    crop,
		"latest":  latest,
		"history": history,
		"source":  "agmarknet",
	}
}

type marketResponse struct {
	Records []struct {
		ModalPrice string `json:"modal_price"`
	} `json:"records"`
}
