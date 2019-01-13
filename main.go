package main

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/lildude/starling"
	"golang.org/x/oauth2"
)

// Settings pulled in from the environment variables.
// SavingGoal is now optional as Starling now does rounding itself, however the Starling API doesn't provide a way to determine this rounding yet.
type Settings struct {
	Port                string `required:"true" envconfig:"PORT"`
	WebhookSecret       string `required:"true" split_words:"true"`
	SavingGoal          string `split_words:"true"`
	PersonalAccessToken string `required:"true" split_words:"true"`
	SweepThreshold      int64  `split_words:"true"`
	SweepSavingGoal     string `split_words:"true"`
}

var s Settings

func main() {
	log.SetFlags(0)
	err := envconfig.Process("starling", &s)
	if err != nil {
		log.Fatal(err.Error())
	}
	// If s.SavingGoal is not set, we don't do rounding
	// If s.SweepSavingGoal is not set, we don't do sweeping
	// No point continuing if neither are set.
	if s.SweepSavingGoal == "" && s.SavingGoal == "" {
		log.Fatal("No savings goal set.")
	}

	http.HandleFunc("/", TxnHandler)
	http.ListenAndServe(":"+s.Port, nil)
}

// TxnHandler handles the incoming webhook event
func TxnHandler(w http.ResponseWriter, r *http.Request) {
	// Return OK as soon as we've received the payload - the webhook doesn't care what we do with the payload so no point holding things back.
	w.WriteHeader(http.StatusOK)

	// Grab body early as we'll need it later
	body, _ := ioutil.ReadAll(r.Body)
	if string(body) == "" {
		log.Println("INFO: empty body, pretending all is OK")
		return
	}

	if !validateSignature(body, r.Header.Get("X-Hook-Signature")) {
		return
	}

	// Parse the contents of web hook payload and log pertinent items for debugging purposes
	wh := new(starling.WebHookPayload)
	err := json.Unmarshal([]byte(body), &wh)
	if err != nil {
		log.Println("ERROR: failed to unmarshal web hook payload:", err)
		return
	}

	// Store the webhook uid in an environment variable and use to try catch duplicate deliveries
	ltu, _ := os.LookupEnv("LAST_TRANSACTION_UID")
	if ltu != "" && ltu == wh.WebhookNotificationUID {
		log.Println("INFO: ignoring duplicate webhook delivery")
		return
	}

	os.Setenv("LAST_TRANSACTION_UID", wh.WebhookNotificationUID)

	log.Println("INFO: type:", wh.WebhookType)
	log.Printf("INFO: amount : %d pence", wh.TransactionAmount.MinorUnits)

	// Ignore anything other than card transactions or specific inbound transactions likely to be large payments like salary etc
	if wh.WebhookType != "TRANSACTION_CARD" &&
		wh.WebhookType != "TRANSACTION_MOBILE_WALLET" &&
		wh.WebhookType != "TRANSACTION_FASTER_PAYMENTS_IN" &&
		wh.WebhookType != "TRANSACTION_NOSTRO_DEPOSIT" {
		log.Printf("INFO: ignoring %s transaction\n", wh.WebhookType)
		return
	}

	var ra int64
	var prettyRa float64
	var destGoal string

	switch wh.WebhookType {
	case "TRANSACTION_CARD", "TRANSACTION_MOBILE_WALLET":
		// Return early if no savings goal
		if s.SavingGoal == "" {
			log.Println("INFO: no roundup savings goal set. Nothing to do.")
			return
		}
		destGoal = s.SavingGoal
		if wh.Direction != "PAYMENT" {
			log.Printf("INFO: ignoring inbound %s transaction\n", wh.WebhookType)
			return
		}
		// Round up to the nearest major unit
		ra = roundUp(wh.TransactionAmount.MinorUnits)
		prettyRa = float64(ra) / 100
		log.Println("INFO: round-up yields:", ra)

	case "TRANSACTION_FASTER_PAYMENTS_IN", "TRANSACTION_NOSTRO_DEPOSIT":
		// Return early if no savings goal
		if s.SweepSavingGoal == "" {
			log.Println("INFO: no sweep savings goal set. Nothing to do.")
			return
		}
		destGoal = s.SweepSavingGoal
		if s.SweepThreshold <= 0 || wh.TransactionAmount.MinorUnits < s.SweepThreshold {
			log.Printf("INFO: ignoring inbound transaction below sweep threshold (%d)\n", s.SweepThreshold)
			return
		}

		if wh.TransactionAmount.MinorUnits > s.SweepThreshold {
			log.Printf("INFO: threshold: %d\n", s.SweepThreshold)
			ra = getBalanceBefore(wh.TransactionAmount.MinorUnits)
			prettyRa = float64(ra) / 100
			log.Printf("INFO: balance before: %.2f\n", prettyRa)
		}
	}

	// Don't try and transfer a zero value to the savings goal
	if ra == 0 {
		log.Println("INFO: nothing to transfer")
		return
	}

	ctx := context.Background()
	cl := newClient(ctx, s.PersonalAccessToken)
	amt := starling.Amount{
		MinorUnits: ra,
		Currency:   wh.SourceAmount.Currency,
	}

	// Transfer the funds to the savings goal
	txn, resp, err := cl.TransferToSavingsGoal(ctx, destGoal, amt)
	if err != nil {
		log.Println("ERROR: failed to move money to savings goal:", err)
		log.Println("ERROR: Starling Bank API returned:", resp.Status)
		return
	}

	log.Printf("INFO: transfer successful (Txn: %s | %.2f)", txn, prettyRa)
	return
}

func newClient(ctx context.Context, token string) *starling.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	baseURL, _ := url.Parse(starling.ProdURL)
	opts := starling.ClientOptions{BaseURL: baseURL}
	return starling.NewClientWithOptions(tc, opts)
}

// Calculate the request signature and reject the request if it doesn't match the signature header
func validateSignature(body []byte, reqSig string) bool {

	// Allow skipping verification - only use during testing
	_, skipSig := os.LookupEnv("SKIP_SIG")
	if skipSig {
		log.Println("INFO: skipping signature verification")
		return true
	}

	sha512 := sha512.New()
	sha512.Write([]byte(s.WebhookSecret + string(body)))
	recSig := base64.StdEncoding.EncodeToString(sha512.Sum(nil))
	if reqSig != recSig {
		log.Println("WARN: reqSig", reqSig)
		log.Println("WARN: recSig", recSig)
		log.Println("ERROR: invalid request signature received")
		return false
	}
	return true
}

func roundUp(txn int64) int64 {
	// By using 99 we ensure that a 0 value is not rounded up to the next 100.
	//amtRound := (math.Round(float64(txn)/100) + 99)
	amtRound := (txn + 99) / 100 * 100
	return int64(amtRound) - txn
}

// Grabs txn deets and removes txn amt from balance and returns the minor units
func getBalanceBefore(txnAmt int64) int64 {
	ctx := context.Background()
	cl := newClient(ctx, s.PersonalAccessToken)
	accounts, _, err := cl.Accounts(ctx)
	if err != nil {
		log.Println("ERROR: problem getting account UID")
		return 0
	}
	bal, _, err := cl.AccountBalance(ctx, accounts[0].UID)
	if err != nil {
		log.Println("ERROR: problem getting balance")
		return 0
	}
	log.Println("INFO: balance: ", bal.Effective.MinorUnits)
	diff := (bal.Effective.MinorUnits - txnAmt)
	return int64(diff)
}
