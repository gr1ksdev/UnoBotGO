package game

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestPrivateViewsAndReadOnlyProjection(t *testing.T) {
	s := testService(t)
	v := position(t, s, uno.BotRules(), false)
	e := s.manager.byID[v.GameID].entry
	before := e.engine.Snapshot()
	for _, id := range []uno.PlayerID{1, 2, 3} {
		got, err := s.PlayerView(t.Context(), Actor{PlayerID: id}, v.GameID)
		if err != nil {
			t.Fatal(err)
		}
		want := before.Players[int(id)-1].Hand
		if len(got.Hand) != len(want) || got.PlayerID != id || got.Public.Revision != before.Revision {
			t.Fatal(got)
		}
		for i, c := range got.Hand {
			if c.Card.ID != want[i] || c.Playable != (e.engine.CanPlay(id, c.Card.ID) == nil) {
				t.Fatal("wrong private projection")
			}
		}
	}
	if _, err := s.PlayerView(t.Context(), Actor{PlayerID: 99}, v.GameID); !errors.Is(err, ErrNotParticipant) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, e.engine.Snapshot()) {
		t.Fatal("view mutated engine")
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{`"Hand"`, `"Cards"`, `"DrawPile"`, `"DiscardPile"`, `"Pending"`, `"State"`, `"a"`, `"b"`, `"c"`, `"draw"`} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("public leak %s: %s", secret, raw)
		}
	}
	// Check the exported API transitively, including nested slices and pointers.
	var inspect func(reflect.Type)
	inspect = func(typ reflect.Type) {
		if typ == reflect.TypeFor[uno.State]() || typ == reflect.TypeFor[uno.Game]() || typ == reflect.TypeFor[uno.Player]() {
			t.Fatalf("snapshot/runtime escaped: %v", typ)
		}
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			inspect(typ.Elem())
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				if typ.Field(i).IsExported() {
					inspect(typ.Field(i).Type)
				}
			}
		}
	}
	api := reflect.TypeFor[*Service]()
	for i := 0; i < api.NumMethod(); i++ {
		method := api.Method(i).Type
		for j := 0; j < method.NumOut(); j++ {
			inspect(method.Out(j))
		}
	}
}

func TestDrawEventsAndReturnedCopies(t *testing.T) {
	s := testService(t)
	v := position(t, s, uno.BotRules(), false)
	out := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.DrawCard})
	found := false
	for _, ev := range out.Events {
		if ev.Type == uno.CardsDrawn {
			found = true
			if ev.CardID != "" || ev.Count != 1 {
				t.Fatal("private draw event", ev)
			}
		}
	}
	if !found {
		t.Fatal("missing draw event")
	}
	own, _ := s.PlayerView(t.Context(), Actor{PlayerID: 1}, v.GameID)
	other, _ := s.PlayerView(t.Context(), Actor{PlayerID: 2}, v.GameID)
	if own.DrawnCardID != "draw" || other.DrawnCardID != "" {
		t.Fatal("draw identity scope")
	}
	public, _ := s.PublicView(t.Context(), v.GameID)
	out.View.Players[0].CardCount = 999
	out.View.Order[0] = 999
	out.View.TopCard.ID = "fake"
	out.Events[0].CardID = "fake"
	own.Hand[0].Card.ID = "fake"
	own.Public.Players[0].ID = 999
	own.Public.Order[0] = 999
	own.Public.TopCard.ID = "fake"
	list, _ := s.FindPlayerGames(t.Context(), Actor{PlayerID: 1})
	list[0].GameID = "fake"
	again, _ := s.PublicView(t.Context(), v.GameID)
	if !reflect.DeepEqual(public, again) {
		t.Fatal("public alias")
	}
	private, _ := s.PlayerView(t.Context(), Actor{PlayerID: 1}, v.GameID)
	if private.Hand[0].Card.ID != "a" {
		t.Fatal("hand alias")
	}
	list, _ = s.FindPlayerGames(t.Context(), Actor{PlayerID: 1})
	if list[0].GameID != v.GameID {
		t.Fatal("summary alias")
	}
	assertIndexes(t, s)
}

func TestClosedSummaryCopies(t *testing.T) {
	s := testService(t)
	v := position(t, s, uno.ClassicRules(), false)
	out := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.PlayCard, CardID: "a"})
	expected, _ := s.PublicView(t.Context(), v.GameID)
	damage := func(v *PublicGameView) {
		v.Players[0].ID = 99
		v.Order[0] = 99
		v.Placements[0].PlayerID = 99
		v.TopCard.ID = "fake"
	}
	damage(&out.View)
	got, _ := s.PublicView(t.Context(), v.GameID)
	if !reflect.DeepEqual(expected, got) {
		t.Fatal("outcome aliases archive")
	}
	damage(&got)
	got, _ = s.PublicView(t.Context(), v.GameID)
	if !reflect.DeepEqual(expected, got) {
		t.Fatal("archive query aliases archive")
	}
	assertIndexes(t, s)
}
