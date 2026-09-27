package rankingimport

import (
	"github.com/malbs/UnoGoBot/internal/groups"
	"testing"
)

func TestParserUnicodeDuplicatesLastSuffixAndInvalid(t *testing.T) {
	raw := "👩🏽‍💻 Ga\u0301briel\u200b - 25\nGabriel - 25\nGabriel - 12\nNome - 15 - 21\n@Alexandre - 21\ninvalid\nName - -2\nName - 999999999999999999999999"
	r, err := New(42, 1, raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Entries) != 8 || r.Entries[0].ImportedName != "👩🏽‍💻 Ga\u0301briel\u200b" || r.Entries[0].Score != 2500 {
		t.Fatal("Unicode lost", r)
	}
	if r.Entries[1].ID == r.Entries[2].ID || r.Entries[1].Score != 2500 || r.Entries[2].Score != 1200 {
		t.Fatal("duplicates merged")
	}
	if r.Entries[3].ImportedName != "Nome - 15" || r.Entries[3].Score != 2100 {
		t.Fatal("wrong suffix")
	}
	for _, e := range r.Entries[5:] {
		if e.Status != Invalid {
			t.Fatal("invalid line accepted", e)
		}
	}
	again, _ := New(42, 2, raw)
	if again.Hash != r.Hash {
		t.Fatal("unstable source hash")
	}
}
func TestConservativeReconciliation(t *testing.T) {
	r, _ := New(42, 1, "Gabriel - 25\nGabriel - 12\nÚnico 🎮 - 7\n@Alexandre - 21\nUnknown - 3")
	users := []groups.KnownUser{{UserID: 1, DisplayName: "Gabriel"}, {UserID: 2, DisplayName: "Único 🎮"}, {UserID: 3, DisplayName: "Alex", Username: "alexandre"}}
	e := Reconcile(r.Entries, users)
	if e[0].Status != Ambiguous || e[1].Status != Ambiguous || e[0].LinkedUserID != nil || e[1].LinkedUserID != nil {
		t.Fatal("duplicates auto-linked")
	}
	if e[2].Status != Linked || *e[2].LinkedUserID != 2 || e[3].Status != Linked || *e[3].LinkedUserID != 3 || e[4].Status != Unresolved {
		t.Fatal("wrong candidate matches", e)
	}
	r, _ = New(42, 1, "Gabriel - 25")
	users = append(users, groups.KnownUser{UserID: 4, DisplayName: "Gabriel"})
	e = Reconcile(r.Entries, users)
	if e[0].Status != Ambiguous {
		t.Fatal("multiple user candidates linked")
	}
}
func TestNoDoubleClaimViaUsername(t *testing.T) {
	r, _ := New(42, 1, "Gabriel - 25\n@gabriel - 12")
	e := Reconcile(r.Entries, []groups.KnownUser{{UserID: 1, DisplayName: "Gabriel", Username: "gabriel"}})
	for _, entry := range e {
		if entry.Status != Ambiguous {
			t.Fatal("same user claimed twice")
		}
	}
}
