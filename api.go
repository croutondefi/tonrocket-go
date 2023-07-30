package tonrocket

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/shopspring/decimal"
)

type Currency string

const TONCurrency Currency = "TONCOIN"

const WebhookTypeInvoicePay = "invoicePay"

func (c Currency) String() string {
	if c == TONCurrency {
		return "TON"
	}

	return string(c)
}

type InvoiceID struct {
	id string
}

func (f *InvoiceID) String() string {
	return f.id
}

func (f *InvoiceID) UnmarshalJSON(data []byte) error {
	number := regexp.MustCompile("[0-9]+").FindString(string(data))

	if number == "" {
		return errors.New("unable to parse invoice_id")
	}

	f.id = number

	return nil
}

type CreateInvoiceRequest struct {
	Amount        float64  `json:"amount"`
	MinPayment    float64  `json:"minPayment"`
	NumPayments   int      `json:"numPayments"`
	Currency      Currency `json:"currency"`
	Description   string   `json:"description"`
	HiddenMessage string   `json:"hiddenMessage"`
	CallbackURL   string   `json:"callbackUrl"`
	Payload       string   `json:"payload"`
	ExpiredIn     int      `json:"expiredIn"`
}

type Invoice struct {
	ID               InvoiceID       `json:"id"`
	Amount           decimal.Decimal `json:"amount"`
	Description      string          `json:"description"`
	HiddenMessage    string          `json:"hiddenMessage"`
	Payload          string          `json:"payload"`
	CallbackURL      string          `json:"callbackUrl"`
	Currency         Currency        `json:"currency"`
	Created          time.Time       `json:"created"`
	Paid             time.Time       `json:"paid"`
	Status           string          `json:"status"`
	ExpiredIn        int             `json:"expiredIn"`
	Link             string          `json:"link"`
	TotalActivations int             `json:"totalActivations"`
	ActivationsLeft  int             `json:"activationsLeft"`
}

type CreateTransferRequest *Transfer

type Transfer struct {
	ID          int64           `json:"id,omitempty"`
	TransferID  string          `json:"transferId"`
	TgUserID    int64           `json:"tgUserId"`
	Currency    Currency        `json:"currency"`
	Amount      decimal.Decimal `json:"amount"`
	Description string          `json:"description"`
}

type InvoiceWebhookRequest struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      *Invoice  `json:"data"`
}

type AppInfo struct {
	Name        string           `json:"name"`
	FeePercents decimal.Decimal  `json:"feePercents"`
	Balances    []map[string]any `json:"balances"`
}

const (
	AuthHeader    = "Rocket-Pay-Key"
	mainnetApiURL = "https://pay.ton-rocket.com"
	testnetApiURL = "https://pay.ton-rocket.com"
)

type tonrocket struct {
	token       string
	httpClient  *http.Client
	testingMode bool
}

type response struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Errors  []*responseError `json:"errors"`
	Data    any              `json:"data"`
}

type responseError struct {
	Property string `json:"property"`
	Error    string `json:"error"`
}

func ParseWebhookRequest(data []byte) (*InvoiceWebhookRequest, error) {
	var webhookData InvoiceWebhookRequest
	if err := json.Unmarshal(data, &webhookData); err != nil {
		return nil, err
	}

	return &webhookData, nil
}

func (c *tonrocket) getRequestUrl() string {
	if c.testingMode {
		return testnetApiURL
	} else {
		return mainnetApiURL
	}
}

func NewTonrocket(token string) Tonrocket {
	return &tonrocket{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		testingMode: false,
	}
}

type Tonrocket interface {
	CreateInvoice(CreateInvoiceRequest) (*Invoice, error)
	CreateTransfer(CreateTransferRequest) (*Transfer, error)
	AppInfo() (*AppInfo, error)
}

func (t *tonrocket) AppInfo() (*AppInfo, error) {
	var resp = &AppInfo{}
	err := t.getRequest("/app/info", nil, resp)

	return resp, err
}

func (t *tonrocket) CreateTransfer(req CreateTransferRequest) (*Transfer, error) {
	var resp = &Transfer{}

	err := t.postRequest("/app/transfer", req, resp)

	return resp, err
}

func (t *tonrocket) CreateInvoice(req CreateInvoiceRequest) (*Invoice, error) {
	var resp = &Invoice{}

	err := t.postRequest("/tg-invoices", req, resp)

	return resp, err
}

func (t *tonrocket) postRequest(path string, body any, target any) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(body)

	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, t.getRequestUrl()+path, &buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp := &response{
		Data: target,
	}

	return t.makeRequest(req, resp)
}

func (t *tonrocket) getRequest(path string, params url.Values, target any) error {
	req, err := http.NewRequest(http.MethodGet, t.getRequestUrl()+path, nil)
	if err != nil {
		return err
	}

	resp := &response{
		Data: target,
	}

	return t.makeRequest(req, resp)
}

func (t *tonrocket) makeRequest(req *http.Request, target *response) error {
	req.Header.Set(AuthHeader, t.token)
	resp, err := t.httpClient.Do(req)

	if err != nil {
		return fmt.Errorf("error while performing a request: %w", err)
	}

	err = json.NewDecoder(resp.Body).Decode(target)
	if err != nil {
		return err
	}

	if !target.Success {
		var errs string
		for i := range target.Errors {
			errs = errs + fmt.Sprintf("%s: %s ", target.Errors[i].Property, target.Errors[i].Error)
		}
		return fmt.Errorf("error received in response: %s | %s", target.Message, errs)
	}

	return nil
}
