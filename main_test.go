package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	mockhttp "github.com/karupanerura/go-mock-http-response"
)

func mockResponse(statusCode int, headers map[string]string, body []byte) {
	http.DefaultClient = mockhttp.NewResponseMock(statusCode, headers, body).MakeClient()
}

func TestRoundUp(t *testing.T) {
	//t.Parallel()
	testCases := []struct {
		name string
		in   int64
		out  int64
	}{
		{"roundup 99", 99, 1},
		{"roundup 1", 1, 99},
		{"roundup 0", 0, 0},
		{"roundup 100", 100, 0},
		{"roundup 2499", 2499, 1},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			//t.Parallel()
			total := roundUp(tc.in)
			if total != tc.out {
				t.Errorf("%v failed, got: %d, want: %d.", tc.name, total, tc.out)
			}
		})
	}
}

func TestValidateSignature(t *testing.T) {
	//t.Parallel()
	// Discard logging info
	log.SetOutput(ioutil.Discard)
	body := []byte(`{"test":"body"}`)
	signature := "C3zcs4qlrazPXGdPacksD/RhFeqBIjm/YkOjvZPo28OxJaUgaZT3RoTuJyGmlJkDWz/viPyWJvTJLbRz2tE7ww=="
	testCases := []struct {
		name string
		body []byte
		sig  string
		res  bool
	}{
		{
			name: "valid signature and body",
			body: body,
			sig:  signature,
			res:  true,
		},
		{
			name: "invalid signature",
			body: body,
			sig:  "foobar",
			res:  false,
		},
		{
			name: "invalid body",
			body: []byte(`{"foo":"bar"}`),
			sig:  signature,
			res:  false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			//t.Parallel()
			if validateSignature(tc.body, tc.sig) != tc.res {
				t.Errorf("%v failed", tc.name)
			}
		})
	}
}

func TestTxnHandler(t *testing.T) {
	//t.Parallel()
	testCases := []struct {
		name      string
		method    string
		body      string
		goal      string
		message   string
		mockresp  []byte
		signature string
	}{
		{
			name:      "empty GET",
			method:    http.MethodGet,
			body:      "",
			goal:      "round",
			message:   "INFO: empty body",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "empty POST",
			method:    http.MethodPost,
			body:      "",
			goal:      "round",
			message:   "INFO: empty body",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "invalid json",
			method:    http.MethodPost,
			body:      `{"foo":"bar}`,
			goal:      "round",
			message:   "ERROR: failed to unmarshal web hook payload",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "non-card outgoing transaction",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_DIRECT_DEBIT"}`,
			goal:      "round",
			message:   "INFO: ignoring TRANSACTION_DIRECT_DEBIT transaction",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "card outbound transaction",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_CARD","direction":"PAYMENT","transactionAmount": {"minorUnits": 2499}}`,
			goal:      "round",
			message:   "INFO: transfer successful (Txn: 12345-67890 | 0.01)",
			mockresp:  []byte(`{"transferUid":"12345-67890","success":true,"errors":[]}`),
			signature: "",
		},
		{
			name:      "no roundup goal set",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_CARD","transactionAmount": {"minorUnits": 2499}}`,
			goal:      "",
			message:   "INFO: no roundup savings goal set. Nothing to do.",
			mockresp:  []byte(`{"transferUid":"12345-67890","success":true,"errors":[]}`),
			signature: "",
		},
		{
			name:      "card inbound transaction",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_CARD","direction":"refund","transactionAmount": {"minorUnits": 2499}}`,
			goal:      "round",
			message:   "INFO: ignoring inbound TRANSACTION_CARD transaction",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "card nothing to roundup",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_CARD","direction":"PAYMENT","transactionAmount": {"minorUnits": 100}}`,
			goal:      "round",
			message:   "INFO: nothing to transfer",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "mobile wallet outbound transaction",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_MOBILE_WALLET","direction":"PAYMENT","transactionAmount":{"minorUnits": 2499}}`,
			goal:      "round",
			message:   "INFO: transfer successful (Txn: 12345-67890 | 0.01)",
			mockresp:  []byte(`{"transferUid":"12345-67890","success":true,"errors":[]}`),
			signature: "",
		},
		{
			name:      "mobile wallet inbound transaction",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_MOBILE_WALLET","direction":"refund","transactionAmount":{"minorUnits": 2499}}`,
			goal:      "round",
			message:   "INFO: ignoring inbound TRANSACTION_MOBILE_WALLET transaction",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "mobile wallet nothing to roundup",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_MOBILE_WALLET","direction":"PAYMENT","transactionAmount":{"minorUnits": 100}}`,
			goal:      "round",
			message:   "INFO: nothing to transfer",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "non-card inbound above threshold",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_FASTER_PAYMENTS_IN","direction":"refund","transactionAmount":{"minorUnits": 250000}}`,
			goal:      "sweep",
			message:   "INFO: transfer successful (Txn:  | 254.12)",
			mockresp:  []byte(`{"accounts":[{"accountUid": "b0b20c9d-3b6b-42f1-a7d0-e70d4538e0d9"}],"effectiveBalance":{"minorUnits": 275412}}`),
			signature: "",
		},
		{
			name:      "no sweep goal set",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_FASTER_PAYMENTS_IN","transactionAmount":{"minorUnits": 250000}}`,
			goal:      "",
			message:   "INFO: no sweep savings goal set. Nothing to do.",
			mockresp:  []byte(`{"accounts":[{"accountUid": "b0b20c9d-3b6b-42f1-a7d0-e70d4538e0d9"}],effectiveBalance":{"minorUnits":  275412}}`),
			signature: "",
		},
		{
			name:      "non-card inbound below threshold",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_FASTER_PAYMENTS_IN","transactionAmount":{"minorUnits": 50000}}`,
			goal:      "sweep",
			message:   "INFO: ignoring inbound transaction below sweep threshold",
			mockresp:  []byte(`{"accounts":[{"accountUid": "b0b20c9d-3b6b-42f1-a7d0-e70d4538e0d9"}],"amount": 500.00, "balance": 754.12}`),
			signature: "",
		},
		{
			name:      "card duplicate webhook delivery 1",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_CARD","direction":"PAYMENT","webhookNotificationUid":"test-trans-uid","transactionAmount":{"minorUnits": 2499}}`,
			goal:      "round",
			message:   "INFO: transfer successful",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "card duplicate webhook delivery 2",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_CARD","direction":"PAYMENT","webhookNotificationUid":"test-trans-uid","transactionAmount":{"minorUnits": 2499}}`,
			goal:      "round",
			message:   "INFO: ignoring duplicate webhook delivery",
			mockresp:  []byte{},
			signature: "",
		},
		{
			name:      "bad signature",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_CARD","webhookNotificationUid":"test-trans-uid","transactionAmount":{"minorUnits": 2499}}`,
			goal:      "round",
			message:   "ERROR: invalid request signature receive",
			mockresp:  []byte{},
			signature: "12345",
		},
		{
			name:      "forced failure to get UID",
			method:    http.MethodPost,
			body:      `{"webhookType":"TRANSACTION_FASTER_PAYMENTS_IN","transactionAmount":{"minorUnits": 250000}}`,
			goal:      "sweep",
			message:   "ERROR: problem getting account UID",
			mockresp:  []byte(`{"broken": "json`),
			signature: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			//t.Parallel()
			// Skip signature verification
			if tc.signature == "" {
				os.Setenv("SKIP_SIG", "1")
			} else {
				os.Unsetenv("SKIP_SIG")
			}
			s.SavingGoal = ""
			s.SweepSavingGoal = ""
			if tc.goal == "round" {
				s.SavingGoal = "round"
			}
			if tc.goal == "sweep" {
				s.SweepSavingGoal = "sweep"
				s.SweepThreshold = 100000
			}
			// Set a mock response, if needed.
			if len(tc.mockresp) > 0 {
				mockResponse(http.StatusOK, map[string]string{"Content-Type": "application/json"}, tc.mockresp)
			}
			// Use a faux logger so we can parse the content to find our debug messages to confirm our tests
			var fauxLog bytes.Buffer
			log.SetOutput(&fauxLog)
			req := httptest.NewRequest(tc.method, "/", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(TxnHandler)
			handler.ServeHTTP(rr, req)
			if !strings.Contains(fauxLog.String(), tc.message) {
				t.Errorf("'%v' failed.\nGot:\n%v\nExpected:\n%v", tc.name, fauxLog.String(), tc.message)
			}
		})
	}
}
