package discordbot

import "testing"

func TestCommandCatalogMatchesHandoff(t *testing.T) {
	want := map[string]bool{
		"panel": true, "rolepanel": true, "verifypanel": true, "testwelcome": true,
		"tickets": true, "ticket-status": false, "ticket-handoff": true, "ticket-resolution": true,
		"whoami": false, "safety": false, "lookup": true, "withdraw": false, "mutasi": false,
		"seller-approve": true, "withdraw-paid": true, "rolesync": true, "guildsync": true,
		"automodsync": true,
		"memberpanel": true, "membersync": true, "areapanel": true,
		"server": true, "revenue": true,
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

// TestAdminOnlyNamesCoversEveryAdminCommand pins the runtime authorization gate
// to the catalog.
//
// DefaultMemberPermissions only hides an admin command by default — a guild
// owner can re-grant it per role, and Discord then delivers the interaction to
// a non-admin. The handleCommand gate is what actually enforces admin, so it
// must name every AdminOnly command. When /automodsync was added the catalog and
// the inline gate drifted, and the command ran with no admin check at all.
func TestAdminOnlyNamesCoversEveryAdminCommand(t *testing.T) {
	for _, c := range Commands {
		if c.AdminOnly && !adminOnlyNames[c.Name] {
			t.Errorf("%s is AdminOnly in the catalog but the runtime gate would let a non-admin run it", c.Name)
		}
		if !c.AdminOnly && adminOnlyNames[c.Name] {
			t.Errorf("%s is not AdminOnly in the catalog but the runtime gate blocks it", c.Name)
		}
	}
	if len(adminOnlyNames) == 0 {
		t.Fatal("runtime admin gate is empty")
	}
}
