// backend/internal/services/billingo_service.go
package services

import (
	"bytes"
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// billingoBaseURL is Billingo's production API base. Verified 2026-09-17
// against Billingo's official OpenAPI v3 (v3.0.14) schema, cross-checked via
// the community-maintained Python swagger-codegen client generated from it:
// https://github.com/kopridar/billingo-v3-python (base URL, X-API-KEY auth
// header, /partners and /documents paths and payload field names all
// confirmed against that schema).
const billingoBaseURL = "https://api.billingo.hu/v3"

type BillingoService struct {
	httpClient *http.Client
}

func NewBillingoService() *BillingoService {
	return &BillingoService{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *BillingoService) loadSettings() (*models.BillingoSettings, error) {
	var settings models.BillingoSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return nil, fmt.Errorf("billingo settings not configured: %w", err)
	}
	if settings.APIKey == "" {
		return nil, fmt.Errorf("billingo API key is not configured")
	}
	return &settings, nil
}

// doRequest sends an authenticated request to Billingo's API and returns the
// raw response body on 2xx. apiBaseURL is a parameter (not the billingoBaseURL
// constant) so tests can point it at an httptest.Server.
func (s *BillingoService) doRequest(apiBaseURL, apiKey, method, path string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encoding billingo request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, apiBaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("building billingo request: %w", err)
	}
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling billingo: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading billingo response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("billingo returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// billingoPartnerAddress mirrors Billingo's "Address" schema. Field name
// verified as "post_code" (not "postal_code") against Billingo's v3 OpenAPI
// schema.
type billingoPartnerAddress struct {
	CountryCode string `json:"country_code"`
	PostCode    string `json:"post_code"`
	City        string `json:"city"`
	Address     string `json:"address"`
}

// billingoPartnerPayload mirrors Billingo's "Partner" schema (name, address,
// emails, taxcode fields verified against Billingo's v3 OpenAPI schema).
type billingoPartnerPayload struct {
	Name    string                 `json:"name"`
	Emails  []string               `json:"emails,omitempty"`
	TaxCode string                 `json:"taxcode,omitempty"`
	Address billingoPartnerAddress `json:"address"`
}

type billingoPartnerResponse struct {
	ID int `json:"id"`
}

// EnsurePartner returns the client's Billingo partner id, creating the
// partner in Billingo (and persisting the id on the client row) if none
// exists yet.
func (s *BillingoService) EnsurePartner(client *models.Client) (string, error) {
	if client.BillingoPartnerID != "" {
		return client.BillingoPartnerID, nil
	}

	settings, err := s.loadSettings()
	if err != nil {
		return "", err
	}

	payload := billingoPartnerPayload{
		Name:    client.Name,
		TaxCode: client.TaxNumber,
		Address: billingoPartnerAddress{
			CountryCode: "HU",
			PostCode:    client.BillingZip,
			City:        client.BillingCity,
			Address:     client.BillingAddress,
		},
	}
	if client.Email != "" {
		payload.Emails = []string{client.Email}
	}

	respBody, err := s.doRequest(billingoBaseURL, settings.APIKey, http.MethodPost, "/partners", payload)
	if err != nil {
		return "", fmt.Errorf("creating billingo partner: %w", err)
	}

	var parsed billingoPartnerResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parsing billingo partner response: %w", err)
	}

	partnerID := fmt.Sprintf("%d", parsed.ID)
	if err := database.GetDB().Model(client).Update("billingo_partner_id", partnerID).Error; err != nil {
		return "", fmt.Errorf("saving billingo partner id: %w", err)
	}
	client.BillingoPartnerID = partnerID

	return partnerID, nil
}

// billingoInvoiceItemPayload mirrors (a subset of) Billingo's "DocumentItem"
// schema. The unit-price field is named "net_unit_amount" in Billingo's real
// schema (not "unit_price" as an earlier draft of this file assumed).
type billingoInvoiceItemPayload struct {
	Name          string  `json:"name"`
	Quantity      float64 `json:"quantity"`
	NetUnitAmount float64 `json:"net_unit_amount"`
	Vat           string  `json:"vat"`
}

// billingoInvoicePayload mirrors (a subset of) Billingo's "DocumentInsert"
// schema. currency, fulfillment_date and due_date are required fields on the
// real API that an earlier draft of this file omitted; there is no
// "should_send"-style field on document creation itself in the real
// schema — emailing an existing document is a separate
// POST /documents/{id}/send call, which this service intentionally does not
// make (see CreateInvoice's doc comment).
type billingoInvoicePayload struct {
	PartnerID       int                          `json:"partner_id"`
	BlockID         int                          `json:"block_id"`
	Type            string                       `json:"type"`
	PaymentMethod   string                       `json:"payment_method"`
	Language        string                       `json:"language"`
	Currency        string                       `json:"currency"`
	FulfillmentDate string                       `json:"fulfillment_date"`
	DueDate         string                       `json:"due_date"`
	Items           []billingoInvoiceItemPayload `json:"items"`
}

type billingoInvoiceResponse struct {
	ID            int    `json:"id"`
	InvoiceNumber string `json:"invoice_number"`
}

// invoiceDueDays is the default payment term (in days) applied to invoices
// created by this service. Billingo requires a due_date; there is currently
// no per-invoice input for this from callers of CreateInvoice.
const invoiceDueDays = 8

// CreateInvoice finalizes (does not send/email) an invoice in Billingo for
// the given partner and net amount, applying Hungary's standard 27% VAT.
// It does not call Billingo's separate "send document" endpoint, so the
// created invoice is not emailed to the partner.
func (s *BillingoService) CreateInvoice(partnerID string, amount float64, description string) (invoiceID, invoiceNumber string, err error) {
	settings, err := s.loadSettings()
	if err != nil {
		return "", "", err
	}
	if settings.BlockID == "" {
		return "", "", fmt.Errorf("billingo block id is not configured")
	}

	var partnerIDInt, blockIDInt int
	if _, err := fmt.Sscanf(partnerID, "%d", &partnerIDInt); err != nil {
		return "", "", fmt.Errorf("invalid billingo partner id %q: %w", partnerID, err)
	}
	if _, err := fmt.Sscanf(settings.BlockID, "%d", &blockIDInt); err != nil {
		return "", "", fmt.Errorf("invalid billingo block id %q: %w", settings.BlockID, err)
	}

	now := time.Now()
	payload := billingoInvoicePayload{
		PartnerID:       partnerIDInt,
		BlockID:         blockIDInt,
		Type:            "invoice",
		PaymentMethod:   "wire_transfer",
		Language:        "hu",
		Currency:        "HUF",
		FulfillmentDate: now.Format("2006-01-02"),
		DueDate:         now.AddDate(0, 0, invoiceDueDays).Format("2006-01-02"),
		Items: []billingoInvoiceItemPayload{
			{Name: description, Quantity: 1, NetUnitAmount: amount, Vat: "27%"},
		},
	}

	respBody, err := s.doRequest(billingoBaseURL, settings.APIKey, http.MethodPost, "/documents", payload)
	if err != nil {
		return "", "", fmt.Errorf("creating billingo invoice: %w", err)
	}

	var parsed billingoInvoiceResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", "", fmt.Errorf("parsing billingo invoice response: %w", err)
	}

	return fmt.Sprintf("%d", parsed.ID), parsed.InvoiceNumber, nil
}
