package ticket

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	refRe    = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._/-]*$`)
	ibanRe   = regexp.MustCompile(`^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$`)
	ethRe    = regexp.MustCompile(`^0X[A-F0-9]{40,}$`)
	btcRe    = regexp.MustCompile(`^(?:BC1|[13])[A-Z0-9]{25,90}$`)
	digitsRe = regexp.MustCompile(`^\d{8,}$`)
	methodRe = regexp.MustCompile(`^[\p{L}][\p{L} .&()/-]*$`)
)

type Withdrawal struct {
	Version       int              `json:"version"`
	SellerID      string           `json:"sellerId"`
	AmountMinor   int64            `json:"amountMinor"`
	Currency      string           `json:"currency"`
	SaleReference string           `json:"saleReference"`
	Method        string           `json:"method"`
	Approval      *ActorTime       `json:"approval,omitempty"`
	Outgoing      *OutgoingPayment `json:"outgoing,omitempty"`
}

type ActorTime struct {
	By string `json:"by"`
	At string `json:"at"`
}

type OutgoingPayment struct {
	ID          string `json:"id"`
	TicketID    string `json:"ticketId"`
	Direction   string `json:"direction"`
	SellerID    string `json:"sellerId"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	Reference   string `json:"reference"`
	By          string `json:"by"`
	At          string `json:"at"`
}

type WithdrawalForm struct {
	Region        string
	TransactionID string
	Amount        string
	Currency      string
	Method        string
}

func currencyExponent(code string) (int, bool) {
	if !ValidCurrency(code) {
		return 0, false
	}
	if ZeroDecimalCurrencies[code] {
		return 0, true
	}
	return 2, true
}

func FormatAmount(amountMinor int64, currency string) string {
	exp, ok := currencyExponent(currency)
	if !ok || amountMinor <= 0 {
		return ""
	}
	if exp == 0 {
		return strconv.FormatInt(amountMinor, 10)
	}
	padded := fmt.Sprintf("%03d", amountMinor)
	return padded[:len(padded)-2] + "." + padded[len(padded)-2:]
}

func ParseAmount(value, code string) (amountMinor int64, currency, display string, err error) {
	currency = strings.ToUpper(strings.TrimSpace(code))
	exp, ok := currencyExponent(currency)
	if !ok || !regexp.MustCompile(`^[A-Za-z]{3}$`).MatchString(strings.TrimSpace(code)) {
		return 0, "", "", ErrWithdrawalInvalid
	}
	amount := strings.TrimSpace(value)
	if amount == "" || len(amount) > 40 {
		return 0, "", "", ErrWithdrawalInvalid
	}
	pattern := `^(?:0|[1-9]\d*)$`
	if exp == 2 {
		pattern = `^(?:0|[1-9]\d*)(?:\.\d{1,2})?$`
	}
	if !regexp.MustCompile(pattern).MatchString(amount) {
		return 0, "", "", ErrWithdrawalInvalid
	}
	parts := strings.Split(amount, ".")
	whole := parts[0]
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	frac = frac + strings.Repeat("0", exp-len(frac))
	combined := whole + frac
	n, convErr := strconv.ParseInt(combined, 10, 64)
	if convErr != nil || n <= 0 || n > math.MaxInt64 {
		return 0, "", "", ErrWithdrawalInvalid
	}
	return n, currency, FormatAmount(n, currency), nil
}

func ValidatePaymentReference(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" || len(raw) > PaymentRefLimit+2 {
		return "", ErrWithdrawalInvalid
	}
	ref := strings.ToUpper(raw)
	if len(ref) > PaymentRefLimit || !refRe.MatchString(ref) {
		return "", ErrWithdrawalInvalid
	}
	if len(SensitiveFindings(map[string]string{"reference": ref})) > 0 {
		return "", ErrSensitive
	}
	stripped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		if r == '.' || r == '_' || r == '/' || r == '-' {
			return -1
		}
		return r
	}, ref)
	if digitsRe.MatchString(stripped) || ibanRe.MatchString(ref) || ethRe.MatchString(ref) || btcRe.MatchString(ref) {
		return "", ErrWithdrawalInvalid
	}
	return ref, nil
}

func ValidateWithdrawalInput(data map[string]string) (map[string]string, Withdrawal, error) {
	keys := []string{"region", "transactionId", "amount", "currency", "method"}
	if len(data) != len(keys) {
		return nil, Withdrawal{}, ErrInvalidForm
	}
	for _, k := range keys {
		if _, ok := data[k]; !ok {
			return nil, Withdrawal{}, ErrInvalidForm
		}
	}
	region := strings.ToUpper(strings.TrimSpace(data["region"]))
	if !ValidRegion(region) {
		return nil, Withdrawal{}, ErrInvalidRegion
	}
	minor, currency, display, err := ParseAmount(data["amount"], data["currency"])
	if err != nil {
		return nil, Withdrawal{}, err
	}
	sale, err := ValidatePaymentReference(data["transactionId"])
	if err != nil {
		return nil, Withdrawal{}, err
	}
	method := strings.TrimSpace(data["method"])
	if method == "" || len(method) > MethodLimit || !methodRe.MatchString(method) || strings.Contains(method, "://") {
		return nil, Withdrawal{}, ErrWithdrawalInvalid
	}
	if len(SensitiveFindings(map[string]string{"method": method})) > 0 {
		return nil, Withdrawal{}, ErrSensitive
	}
	clean := map[string]string{
		"region": region, "transactionId": sale, "amount": display, "currency": currency, "method": method,
	}
	wd := Withdrawal{Version: 1, AmountMinor: minor, Currency: currency, SaleReference: sale, Method: method}
	return clean, wd, nil
}

func IsWithdrawal(typeKey, workflow string) bool {
	return typeKey == "withdraw" && workflow == WorkflowWithdrawal
}

func rfc3339Milli(value string) bool {
	t, err := time.Parse("2006-01-02T15:04:05.000Z07:00", value)
	return err == nil && t.UTC().Format("2006-01-02T15:04:05.000Z") == value
}

func actorTimeOK(at ActorTime, sellerID string) bool {
	return at.By != "" && at.By != sellerID && rfc3339Milli(at.At)
}

func ApproveWithdrawal(rec Record, actorID, at string) (Record, error) {
	if !IsWithdrawal(rec.Type, rec.Workflow) || rec.Status != "open" || rec.Withdrawal == nil {
		return Record{}, ErrWithdrawalInvalid
	}
	if rec.Withdrawal.Approval != nil {
		return Record{}, ErrAlreadyApproved
	}
	stamp := ActorTime{By: actorID, At: at}
	if !actorTimeOK(stamp, rec.OpenerID) {
		return Record{}, ErrNotAdmin
	}
	opened, err := time.Parse(time.RFC3339Nano, rec.OpenedAt)
	if err != nil {
		opened, err = time.Parse(time.RFC3339, rec.OpenedAt)
	}
	paidAt, err2 := time.Parse(time.RFC3339Nano, at)
	if err2 != nil {
		paidAt, err2 = time.Parse(time.RFC3339, at)
	}
	if err != nil || err2 != nil || paidAt.Before(opened) {
		return Record{}, ErrWithdrawalInvalid
	}
	next := rec
	wd := *rec.Withdrawal
	wd.Approval = &stamp
	next.Withdrawal = &wd
	next.Status = "approved"
	next.History = append(append([]Event(nil), rec.History...), Event{At: at, By: actorID, Event: "status:open->approved"})
	return next, nil
}

func RecordWithdrawalPayment(rec Record, actorID, at, reference string, others []Record) (Record, error) {
	if !IsWithdrawal(rec.Type, rec.Workflow) || rec.Withdrawal == nil {
		return Record{}, ErrNotApproved
	}
	if rec.Withdrawal.Outgoing != nil {
		return Record{}, ErrAlreadyPaid
	}
	if rec.Status != "approved" {
		return Record{}, ErrNotApproved
	}
	ref, err := ValidatePaymentReference(reference)
	if err != nil {
		return Record{}, err
	}
	for _, other := range others {
		if other.Withdrawal != nil && other.Withdrawal.Outgoing != nil && other.Withdrawal.Outgoing.Reference == ref {
			return Record{}, ErrDuplicatePayment
		}
	}
	stamp := ActorTime{By: actorID, At: at}
	if !actorTimeOK(stamp, rec.OpenerID) {
		return Record{}, ErrNotAdmin
	}
	out := OutgoingPayment{
		ID: "OUT-" + rec.ID, TicketID: rec.ID, Direction: "outgoing", SellerID: rec.OpenerID,
		AmountMinor: rec.Withdrawal.AmountMinor, Currency: rec.Withdrawal.Currency,
		Reference: ref, By: actorID, At: at,
	}
	next := rec
	wd := *rec.Withdrawal
	wd.Outgoing = &out
	next.Withdrawal = &wd
	next.Status = "paid"
	next.History = append(append([]Event(nil), rec.History...), Event{At: at, By: actorID, Event: "withdraw.paid", MutationID: out.ID})
	return next, nil
}
