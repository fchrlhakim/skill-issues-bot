package ticket

import "time"

type Event struct {
	At         string `json:"at"`
	Event      string `json:"event"`
	By         string `json:"by"`
	Reason     string `json:"reason,omitempty"`
	MemberID   string `json:"memberId,omitempty"`
	MutationID string `json:"mutationId,omitempty"`
}

type Resolution struct {
	Text string `json:"text"`
	By   string `json:"by"`
	At   string `json:"at"`
}

type SellerApproval struct {
	Version  int    `json:"version"`
	By       string `json:"by"`
	At       string `json:"at"`
	MemberID string `json:"memberId"`
}

type Record struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Code           string            `json:"code"`
	Region         string            `json:"region"`
	Workflow       string            `json:"workflow"`
	OpenerID       string            `json:"openerId"`
	OpenerTag      string            `json:"openerTag,omitempty"`
	ThreadID       string            `json:"threadId"`
	Status         string            `json:"status"`
	AssignedTo     *string           `json:"assignedTo"`
	OpenedAt       string            `json:"openedAtUtc"`
	ClosedAt       string            `json:"closedAtUtc,omitempty"`
	SummaryMsgID   string            `json:"summaryMessageId,omitempty"`
	Data           map[string]string `json:"data"`
	History        []Event           `json:"history"`
	Resolution     *Resolution       `json:"resolution,omitempty"`
	Withdrawal     *Withdrawal       `json:"withdrawal,omitempty"`
	SellerApproval *SellerApproval   `json:"sellerApproval,omitempty"`
}

func (r Record) Closed() bool {
	return r.Status == "closed"
}

func ActiveLimit(existing []Record, openerID, typeKey string) error {
	active := 0
	for _, t := range existing {
		if t.OpenerID != openerID || t.Closed() {
			continue
		}
		if t.Type == typeKey {
			return ErrDuplicateType
		}
		active++
	}
	if active >= MaxOpenTickets {
		return ErrOpenLimit
	}
	return nil
}

func NewRecord(id, typeKey, code, region, workflow, openerID, openerTag, threadID string, data map[string]string, wd *Withdrawal, now time.Time) Record {
	at := now.UTC().Format("2006-01-02T15:04:05.000Z")
	rec := Record{
		ID: id, Type: typeKey, Code: code, Region: region, Workflow: workflow,
		OpenerID: openerID, OpenerTag: openerTag, ThreadID: threadID,
		Status: "open", OpenedAt: at, Data: data,
		History: []Event{{At: at, Event: "created", By: openerID}},
	}
	if wd != nil {
		copyWD := *wd
		copyWD.SellerID = openerID
		rec.Withdrawal = &copyWD
	}
	return rec
}

func Claim(rec Record, actorID string, now time.Time) (Record, error) {
	if rec.AssignedTo != nil && *rec.AssignedTo != "" {
		if *rec.AssignedTo == actorID {
			return rec, ErrAlreadyClaimed
		}
		return rec, ErrAlreadyClaimed
	}
	id := actorID
	rec.AssignedTo = &id
	rec.History = append(rec.History, Event{At: now.UTC().Format("2006-01-02T15:04:05.000Z"), Event: "claimed", By: actorID})
	return rec, nil
}

func Handoff(rec Record, actorID, targetID, reason string, now time.Time) (Record, error) {
	if targetID == "" || targetID == rec.OpenerID || targetID == actorID {
		return rec, ErrHandoffTarget
	}
	text, err := ValidateTicketText(reason)
	if err != nil {
		return rec, err
	}
	id := targetID
	rec.AssignedTo = &id
	rec.History = append(rec.History, Event{
		At: now.UTC().Format("2006-01-02T15:04:05.000Z"), Event: "handoff", By: actorID, Reason: text, MemberID: targetID,
	})
	return rec, nil
}

func SetResolution(rec Record, actorID, note string, now time.Time) (Record, error) {
	text, err := ValidateTicketText(note)
	if err != nil {
		return rec, err
	}
	at := now.UTC().Format("2006-01-02T15:04:05.000Z")
	rec.Resolution = &Resolution{Text: text, By: actorID, At: at}
	rec.History = append(rec.History, Event{At: at, Event: "resolution", By: actorID})
	return rec, nil
}

func Transition(rec Record, actorID, to string, now time.Time) (Record, error) {
	if !CanTransition(rec.Type, rec.Workflow, rec.Status, to) {
		return rec, ErrIllegalTransition
	}
	if to == "closed" {
		if rec.Resolution == nil || !ValidResolution(rec.Resolution.Text, rec.Resolution.By, rec.Resolution.At) {
			return rec, ErrResolutionMissing
		}
		rec.ClosedAt = now.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	from := rec.Status
	rec.Status = to
	rec.History = append(rec.History, Event{
		At: now.UTC().Format("2006-01-02T15:04:05.000Z"), Event: "status:" + from + "->" + to, By: actorID,
	})
	return rec, nil
}

func ApproveSeller(rec Record, actorID, reason, memberID string, now time.Time) (Record, error) {
	if rec.Type != "seller-verification" || rec.Workflow != WorkflowReview {
		return rec, ErrSellerTicket
	}
	if rec.Status == "closed" || rec.Status == "cancelled" || rec.Status == "resolved" {
		return rec, ErrIllegalTransition
	}
	if rec.SellerApproval != nil {
		return rec, ErrAlreadyApproved
	}
	text, err := ValidateTicketText(reason)
	if err != nil {
		return rec, err
	}
	at := now.UTC().Format("2006-01-02T15:04:05.000Z")
	rec.SellerApproval = &SellerApproval{Version: 1, By: actorID, At: at, MemberID: memberID}
	rec.History = append(rec.History, Event{At: at, Event: "seller.approve", By: actorID, Reason: text, MemberID: memberID})
	return rec, nil
}

func NeedsReminder(rec Record, now time.Time) bool {
	if rec.AssignedTo != nil && *rec.AssignedTo != "" {
		return false
	}
	switch rec.Status {
	case "resolved", "cancelled", "closed", "paid", "rejected":
		return false
	}
	opened, err := time.Parse(time.RFC3339Nano, rec.OpenedAt)
	if err != nil {
		opened, err = time.Parse(time.RFC3339, rec.OpenedAt)
	}
	if err != nil {
		return false
	}
	return now.Sub(opened) >= 30*time.Minute
}
