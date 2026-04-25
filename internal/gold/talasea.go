package gold

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const DefaultGoldAPIURL = "https://api.talasea.ir/api/market/getGoldPrice"

// Quote matches the Talasea getGoldPrice JSON body.
type Quote struct {
	Price     string  `json:"price"`
	Change24h string  `json:"change24h"`
}

// Client fetches the public gold price (no API key in sample).
type Client struct {
	HTTP    *http.Client
	BaseURL string
}

func (c *Client) url() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultGoldAPIURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// FetchQuote returns parsed price and 24h change. The Talasea getGoldPrice
// `price` string is the spot in Toman per milligram of gold. Rial for a
// holding: milligrams × toman per mg × rial per toman (Iran: 10 Rial = 1 Toman).
func (c *Client) FetchQuote() (Quote, error) {
	req, err := http.NewRequest(http.MethodGet, c.url(), nil)
	if err != nil {
		return Quote{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "accounting/1.0")
	res, err := c.httpClient().Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Quote{}, fmt.Errorf("gold api: %s", res.Status)
	}
	var q Quote
	if err := json.NewDecoder(res.Body).Decode(&q); err != nil {
		return Quote{}, err
	}
	if q.Price == "" {
		return Quote{}, fmt.Errorf("gold api: empty price")
	}
	return q, nil
}

// DefaultRialPerToman is the usual Iran conversion (1 Toman = 10 Rial).
const DefaultRialPerToman = 10.0

// RialFromTomanPerMilligram returns total Rial for a gold mass when the
// cached/API unit is Toman per milligram. Optional scale is an extra
// multiplicative factor from config (default 1).
func RialFromTomanPerMilligram(milligrams, tomanPerMg, rialPerToman, scale float64) float64 {
	if rialPerToman <= 0 {
		rialPerToman = DefaultRialPerToman
	}
	if scale <= 0 {
		scale = 1
	}
	return milligrams * tomanPerMg * rialPerToman * scale
}
