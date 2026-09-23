package ticket

import "strings"

const (
	WorkflowLegacy      = "legacy"
	WorkflowWithdrawal  = "withdrawal-v1"
	WorkflowTransaction = "transaction-v1"
	WorkflowReview      = "review-v1"
	WorkflowSupport     = "support-v1"

	OpenerBuyer  = "buyer"
	OpenerSeller = "seller"
	OpenerAny    = "any"

	MaxOpenTickets  = 3
	TicketTextLimit = 900
	PaymentRefLimit = 120
	MethodLimit     = 60
)

type Type struct {
	Key      string
	Code     string
	Label    string
	Workflow string
	Opener   string
	Requires []string
}

type Status struct {
	Key      string
	Label    string
	Next     []string
	Terminal bool
	Note     string
}

type Workflow struct {
	ID       string
	Statuses []Status
}

var RegionCodes = []string{
	"ID", "MY", "SG", "PH", "TH", "VN", "BN",
	"IN", "BD", "PK", "LK",
	"CN", "HK", "TW", "JP", "KR",
	"XX",
}

var Currencies = []string{
	"IDR", "MYR", "SGD", "PHP", "THB", "VND", "BND",
	"INR", "BDT", "PKR", "LKR",
	"CNY", "HKD", "TWD", "JPY", "KRW",
	"USD",
}

var ZeroDecimalCurrencies = map[string]bool{"JPY": true, "KRW": true, "VND": true}

var Types = []Type{
	{Key: "purchase", Code: "BUY", Label: "Purchase Coordination", Workflow: WorkflowTransaction, Opener: OpenerBuyer,
		Requires: []string{"product", "quantity", "price", "currency", "paymentMethod", "region"}},
	{Key: "sale", Code: "SELL", Label: "Sale Coordination", Workflow: WorkflowTransaction, Opener: OpenerSeller,
		Requires: []string{"product", "quantity", "price", "currency", "deliveryMethod", "region"}},
	{Key: "introduction", Code: "INTRO", Label: "Buyer-Seller Introduction", Workflow: WorkflowTransaction, Opener: OpenerAny,
		Requires: []string{"counterparty", "product", "region"}},
	{Key: "seller-verification", Code: "VERIFY", Label: "Seller Verification", Workflow: WorkflowReview, Opener: OpenerBuyer,
		Requires: []string{"region", "languages", "currencies", "productCategories", "deliveryMethod", "refundTerms", "legalityDeclaration"}},
	{Key: "buyer-support", Code: "SUPPORT", Label: "Buyer Support", Workflow: WorkflowSupport, Opener: OpenerBuyer,
		Requires: []string{"summary"}},
	{Key: "delivery", Code: "DELIVERY", Label: "Delivery Confirmation", Workflow: WorkflowTransaction, Opener: OpenerAny,
		Requires: []string{"transactionId", "deliveryStatus"}},
	{Key: "refund", Code: "REFUND", Label: "Refund Request", Workflow: WorkflowReview, Opener: OpenerBuyer,
		Requires: []string{"transactionId", "reason", "currency", "amount"}},
	{Key: "dispute", Code: "DISPUTE", Label: "Dispute Review", Workflow: WorkflowReview, Opener: OpenerAny,
		Requires: []string{"transactionId", "chronology", "evidence", "region", "currency"}},
	{Key: "scam", Code: "SCAM", Label: "Scam Report", Workflow: WorkflowReview, Opener: OpenerAny,
		Requires: []string{"reportedUser", "chronology", "evidence"}},
	{Key: "impersonation", Code: "IMPERSONATE", Label: "Impersonation Report", Workflow: WorkflowReview, Opener: OpenerAny,
		Requires: []string{"reportedUser", "impersonatedParty", "evidence"}},
	{Key: "regional", Code: "REGION", Label: "Regional Support", Workflow: WorkflowSupport, Opener: OpenerAny,
		Requires: []string{"region", "summary"}},
	{Key: "translation", Code: "TRANSLATE", Label: "Translation Support", Workflow: WorkflowSupport, Opener: OpenerAny,
		Requires: []string{"language", "summary"}},
	{Key: "partnership", Code: "PARTNER", Label: "Partnership Request", Workflow: WorkflowSupport, Opener: OpenerAny,
		Requires: []string{"organisation", "proposal"}},
	{Key: "general", Code: "GEN", Label: "General Support", Workflow: WorkflowSupport, Opener: OpenerAny,
		Requires: []string{"summary"}},
	{Key: "withdraw", Code: "WD", Label: "Seller Withdrawal", Workflow: WorkflowWithdrawal, Opener: OpenerSeller,
		Requires: []string{"region", "transactionId", "amount", "currency", "method"}},
	{Key: "campaign", Code: "CAMPAIGN", Label: "Campaign Application", Workflow: WorkflowReview, Opener: OpenerAny,
		Requires: []string{"region", "chronology"}},
	{Key: "affiliate", Code: "AFFILIATE", Label: "Affiliate Application", Workflow: WorkflowReview, Opener: OpenerAny,
		Requires: []string{"region", "chronology"}},
}

var GatedTypes = map[string]bool{
	"withdraw": true, "seller-verification": true, "campaign": true, "affiliate": true,
}

var WithdrawalNotice = "Approval is not payment. An admin must verify the external sale and actual payment, " +
	"then use /withdraw-paid to confirm one outgoing record. No automatic transfer or balance is provided."

func TypeByKey(key string) (Type, bool) {
	for _, t := range Types {
		if t.Key == key {
			return t, true
		}
	}
	return Type{}, false
}

func ValidRegion(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, r := range RegionCodes {
		if r == code {
			return true
		}
	}
	return false
}

func ValidCurrency(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, c := range Currencies {
		if c == code {
			return true
		}
	}
	return false
}
