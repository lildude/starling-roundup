package starling

import (
	"context"
	"net/http"
)

// DirectDebitMandate represents a single mandate
type DirectDebitMandate struct {
	UID            string `json:"uid"`
	Reference      string `json:"reference"`
	Status         string `json:"status"`
	Source         string `json:"source"`
	Created        string `json:"created"`
	Cancelled      string `json:"cancelled"`
	OriginatorName string `json:"originatorName"`
	OriginatorUID  string `json:"originatorUid"`
}

// DirectDebitMandates represents a list of mandates
type directDebitMandates struct {
	Mandates []DirectDebitMandate `json:"mandates"`
}

// HALContacts is a HAL wrapper around the DirectDebitMandates type.
type halDirectDebitMandates struct {
	Links    struct{}             `json:"_links"`
	Embedded *directDebitMandates `json:"_embedded"`
}

// DirectDebitMandates returns the DirectDebitMandates for the current customer.
func (c *Client) DirectDebitMandates(ctx context.Context) ([]DirectDebitMandate, *http.Response, error) {
	req, err := c.NewRequest("GET", "/api/v1/direct-debit/mandates", nil)
	if err != nil {
		return nil, nil, err
	}

	hMandates := new(halDirectDebitMandates)
	resp, err := c.Do(ctx, req, &hMandates)

	if hMandates == nil {
		return nil, resp, err
	}

	if hMandates.Embedded == nil {
		return nil, resp, err
	}

	return hMandates.Embedded.Mandates, resp, err
}

// DirectDebitMandate returns a single DirectDebitMandate for the current customer.
func (c *Client) DirectDebitMandate(ctx context.Context, uid string) (*DirectDebitMandate, *http.Response, error) {
	req, err := c.NewRequest("GET", "/api/v1/direct-debit/mandates/"+uid, nil)
	if err != nil {
		return nil, nil, err
	}

	var mandate *DirectDebitMandate
	resp, err := c.Do(ctx, req, &mandate)
	return mandate, resp, err
}

// DeleteDirectDebitMandate deletes an individual DirectDebitMandate for the current customer.
func (c *Client) DeleteDirectDebitMandate(ctx context.Context, uid string) (*http.Response, error) {
	req, err := c.NewRequest("DELETE", "/api/v1/direct-debit/mandates/"+uid, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(ctx, req, nil)
	return resp, err
}
