package ticket

import (
	"testing"
	"time"
)

// isNullJSON reports whether a jsonb column will be written as SQL NULL. A
// pointer form is NULL only when nil. A plain-string form is never NULL: an
// empty string is written as ” and rejected by Postgres, which is the defect
// this guards. The helper therefore fails on the value, not the type, so the
// test stays meaningful when someone reintroduces the string field.
func isNullJSON(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case *string:
		return v == nil
	case string:
		return false
	}
	return false
}

// Ticket has four optional jsonb columns. Postgres rejects an empty string as
// invalid JSON input (SQLSTATE 22P02), so an absent resolution must be written
// as NULL, not "". When toModel filled them with the empty string, creating any
// ticket without a resolution failed against Postgres - the in-memory
// repository used by the other tests never saw it, because it does not encode
// jsonb at all.
func TestToModelLeavesAbsentJSONColumnsNull(t *testing.T) {
	rec := Record{
		ID:       "GEN-ID-member-00001",
		Type:     "general",
		Code:     "GEN",
		Region:   "ID",
		Workflow: "support-v1",
		Status:   "open",
		OpenerID: "member",
		ThreadID: "GEN-ID-member-00001",
		OpenedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Data:     map[string]string{"summary": "probe"},
	}

	row := toModel(rec)

	for name, value := range map[string]any{
		"resolution_json":      row.ResolutionJSON,
		"withdrawal_json":      row.WithdrawalJSON,
		"seller_approval_json": row.SellerJSON,
	} {
		if !isNullJSON(value) {
			t.Fatalf("%s = %v, want NULL: an empty string is invalid jsonb input", name, value)
		}
	}

	// Required jsonb columns must still be populated.
	if row.DataJSON == "" || row.HistoryJSON == "" {
		t.Fatalf("required json columns were left empty: data=%q history=%q", row.DataJSON, row.HistoryJSON)
	}
}

// The inverse: once a resolution exists it must be persisted, not dropped.
func TestToModelPersistsPresentJSONColumns(t *testing.T) {
	rec := Record{
		ID:       "GEN-ID-member-00002",
		Type:     "withdrawal",
		Code:     "WD",
		Region:   "ID",
		Workflow: "withdrawal-v1",
		Status:   "open",
		OpenerID: "seller",
		ThreadID: "GEN-ID-member-00002",
		OpenedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Resolution: &Resolution{
			Text: "paid out",
			By:   "admin",
			At:   time.Now().UTC().Format(time.RFC3339Nano),
		},
	}

	row := toModel(rec)
	if isNullJSON(row.ResolutionJSON) {
		t.Fatalf("resolution_json = %v, want encoded payload", row.ResolutionJSON)
	}
}
