package discordbot

import "testing"

func TestCommandCatalogMatchesHandoff(t *testing.T) {
	want := map[string]bool{
		"panel": true, "rolepanel": true, "verifypanel": true, "testwelcome": true,
		"tickets": true, "ticket-status": false, "ticket-handoff": true, "ticket-resolution": true,
		"whoami": false, "safety": false, "lookup": true, "withdraw": false, "mutasi": false,
		"seller-approve": true, "withdraw-paid": true, "rolesync": true,
	}
	if len(Commands) != len(want) {
		t.Fatalf("got %d commands", len(Commands))
	}
	for _, c := range Commands {
		admin, ok := want[c.Name]
		if !ok {
			t.Fatalf("unexpected command %s", c.Name)
		}
		if c.AdminOnly != admin {
			t.Fatalf("%s admin=%v", c.Name, c.AdminOnly)
		}
	}
}
