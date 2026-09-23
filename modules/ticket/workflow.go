package ticket

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var controlRunes = regexp.MustCompile("[\u0000-\u0008\u000B-\u001F\u007F\u200B-\u200F\u202A-\u202E\u2066-\u2069]")

func graph(id string, statuses []Status) Workflow {
	cp := make([]Status, len(statuses))
	for i, s := range statuses {
		next := append([]string(nil), s.Next...)
		cp[i] = Status{Key: s.Key, Label: s.Label, Next: next, Terminal: s.Terminal, Note: s.Note}
	}
	return Workflow{ID: id, Statuses: cp}
}

func terminalStatuses() []Status {
	return []Status{
		{Key: "resolved", Label: "Resolved", Next: []string{"closed"}},
		{Key: "cancelled", Label: "Cancelled", Next: []string{"closed"}},
		{Key: "closed", Label: "Closed", Next: nil, Terminal: true},
	}
}

var legacyStatuses = []Status{
	{Key: "open", Label: "Open", Next: []string{"pending-verification", "buyer-confirmation", "seller-confirmation", "cancelled"}},
	{Key: "pending-verification", Label: "Pending Verification", Next: []string{"buyer-confirmation", "seller-confirmation", "cancelled"}},
	{Key: "buyer-confirmation", Label: "Buyer Confirmation Needed", Next: []string{"seller-confirmation", "payment-pending", "cancelled"}},
	{Key: "seller-confirmation", Label: "Seller Confirmation Needed", Next: []string{"buyer-confirmation", "payment-pending", "cancelled"}},
	{Key: "payment-pending", Label: "Payment Pending", Next: []string{"payment-confirmed", "cancelled", "dispute-opened"}},
	{Key: "payment-confirmed", Label: "Payment Confirmed by Relevant Party", Next: []string{"delivery-pending", "dispute-opened"},
		Note: "Confirmed BY THE RELEVANT PARTY — Skillissue.ai does not verify or hold payments."},
	{Key: "delivery-pending", Label: "Delivery Pending", Next: []string{"delivery-submitted", "dispute-opened", "cancelled"}},
	{Key: "delivery-submitted", Label: "Delivery Submitted", Next: []string{"buyer-review", "dispute-opened"}},
	{Key: "buyer-review", Label: "Buyer Review", Next: []string{"resolved", "dispute-opened"}},
	{Key: "dispute-opened", Label: "Dispute Opened", Next: []string{"escalated", "resolved", "cancelled"}},
	{Key: "escalated", Label: "Escalated", Next: []string{"resolved", "cancelled"}},
	{Key: "resolved", Label: "Resolved", Next: []string{"closed"}},
	{Key: "cancelled", Label: "Cancelled", Next: []string{"closed"}},
	{Key: "closed", Label: "Closed", Next: nil, Terminal: true},
}

var workflows = map[string]Workflow{
	WorkflowLegacy: graph(WorkflowLegacy, legacyStatuses),
	WorkflowWithdrawal: graph(WorkflowWithdrawal, []Status{
		{Key: "open", Label: "Awaiting admin review", Next: []string{"approved", "rejected", "cancelled"}, Note: WithdrawalNotice},
		{Key: "approved", Label: "Approved", Next: []string{"rejected", "cancelled"}, Note: WithdrawalNotice},
		{Key: "paid", Label: "Payment confirmed by admin", Next: []string{"closed"}, Note: WithdrawalNotice},
		{Key: "rejected", Label: "Rejected", Next: []string{"closed"}, Note: WithdrawalNotice},
		{Key: "cancelled", Label: "Cancelled", Next: []string{"closed"}, Note: WithdrawalNotice},
		{Key: "closed", Label: "Closed", Next: nil, Terminal: true, Note: WithdrawalNotice},
	}),
	WorkflowTransaction: graph(WorkflowTransaction, legacyStatuses),
	WorkflowReview: graph(WorkflowReview, append([]Status{
		{Key: "open", Label: "Open", Next: []string{"under-admin-review", "cancelled"}},
		{Key: "under-admin-review", Label: "Under admin review", Next: []string{"waiting-for-member", "escalated", "resolved", "cancelled"}},
		{Key: "waiting-for-member", Label: "Waiting for member", Next: []string{"under-admin-review", "escalated", "resolved", "cancelled"}},
		{Key: "escalated", Label: "Escalated", Next: []string{"under-admin-review", "waiting-for-member", "resolved", "cancelled"}},
	}, terminalStatuses()...)),
	WorkflowSupport: graph(WorkflowSupport, append([]Status{
		{Key: "open", Label: "Open", Next: []string{"in-progress", "cancelled"}},
		{Key: "in-progress", Label: "In progress", Next: []string{"waiting-for-member", "resolved", "cancelled"}},
		{Key: "waiting-for-member", Label: "Waiting for member", Next: []string{"in-progress", "resolved", "cancelled"}},
	}, terminalStatuses()...)),
}

func WorkflowForType(typeKey string) string {
	t, ok := TypeByKey(typeKey)
	if !ok {
		return ""
	}
	return t.Workflow
}

func WorkflowForTicket(typeKey, workflowID, status string) (Workflow, bool) {
	if workflowID == "" {
		w, ok := workflows[WorkflowLegacy]
		return w, ok
	}
	expected := WorkflowForType(typeKey)
	if expected != workflowID {
		return Workflow{}, false
	}
	w, ok := workflows[workflowID]
	return w, ok
}

func StatusFor(typeKey, workflowID, status string) (Status, bool) {
	w, ok := WorkflowForTicket(typeKey, workflowID, status)
	if !ok {
		return Status{}, false
	}
	for _, s := range w.Statuses {
		if s.Key == status {
			return s, true
		}
	}
	return Status{}, false
}

func CanTransition(typeKey, workflowID, from, to string) bool {
	s, ok := StatusFor(typeKey, workflowID, from)
	if !ok {
		return false
	}
	for _, n := range s.Next {
		if n == to {
			return true
		}
	}
	return false
}

func NextStatuses(typeKey, workflowID, status string) []Status {
	w, ok := WorkflowForTicket(typeKey, workflowID, status)
	if !ok {
		return nil
	}
	cur, ok := StatusFor(typeKey, workflowID, status)
	if !ok {
		return nil
	}
	out := make([]Status, 0, len(cur.Next))
	for _, key := range cur.Next {
		for _, s := range w.Statuses {
			if s.Key == key {
				out = append(out, s)
			}
		}
	}
	return out
}

func NormalizeText(value string) string {
	return strings.TrimSpace(controlRunes.ReplaceAllString(value, ""))
}

func ValidateTicketText(value string) (string, error) {
	if utf8.RuneCountInString(value) > TicketTextLimit || len(value) > TicketTextLimit {
		return "", ErrTextLength
	}
	text := NormalizeText(value)
	if text == "" {
		return "", ErrTextEmpty
	}
	if len(SensitiveFindings(map[string]string{"text": text})) > 0 {
		return "", ErrSensitive
	}
	return text, nil
}

func ValidResolution(text, by, at string) bool {
	if NormalizeText(text) != text || text == "" || by == "" || at == "" {
		return false
	}
	if _, err := ValidateTicketText(text); err != nil {
		return false
	}
	parsed, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, at)
	}
	return err == nil && !parsed.IsZero()
}
