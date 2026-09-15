package telegram

import (
	"sync"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestTokenStore_NormalConsumption(t *testing.T) {
	fakeNow := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	store := NewTokenStore(100, 10, func() time.Time { return fakeNow }, nil)

	action := uno.Action{
		Type:     uno.PlayCard,
		PlayerID: 42,
		CardID:   "c001",
		Revision: 1,
	}

	tok, err := store.CreateActionToken(42, "g1", 1001, action, 2*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error creating token: %v", err)
	}

	// First consumption: should succeed
	res, status := store.ConsumeAction(tok, 42)
	if status != ConsumeOK {
		t.Fatalf("expected ConsumeOK, got %v", status)
	}
	if res.Action.CardID != "c001" {
		t.Errorf("expected card c001, got %v", res.Action.CardID)
	}

	// Second consumption: should be already consumed
	_, status2 := store.ConsumeAction(tok, 42)
	if status2 != ConsumeAlreadyConsumed {
		t.Fatalf("expected ConsumeAlreadyConsumed, got %v", status2)
	}

	// Set action result wipes card details
	store.SetActionResult(tok, "confirmed")
	res3, status3 := store.ConsumeAction(tok, 42)
	if status3 != ConsumeAlreadyConsumed {
		t.Fatalf("expected ConsumeAlreadyConsumed, got %v", status3)
	}
	if res3.ResultSummary != "confirmed" {
		t.Errorf("expected summary confirmed, got %v", res3.ResultSummary)
	}
	if res3.Action.CardID != "" {
		t.Errorf("expected card ID wiped, got %v", res3.Action.CardID)
	}
}

func TestTokenStore_ExpirationAndUserMismatch(t *testing.T) {
	currentTime := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	store := NewTokenStore(100, 10, func() time.Time { return currentTime }, nil)

	tok, err := store.CreateActionToken(42, "g1", 1001, uno.Action{Type: uno.DrawCard, PlayerID: 42}, 1*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// User mismatch
	_, status := store.ConsumeAction(tok, 99)
	if status != ConsumeUserMismatch {
		t.Fatalf("expected ConsumeUserMismatch, got %v", status)
	}

	// Advance clock past expiration
	currentTime = currentTime.Add(2 * time.Minute)

	_, statusExpired := store.ConsumeAction(tok, 42)
	if statusExpired != ConsumeExpired {
		t.Fatalf("expected ConsumeExpired, got %v", statusExpired)
	}
}

func TestTokenStore_ConcurrentConsumption(t *testing.T) {
	store := NewTokenStore(1000, 100, time.Now, nil)

	tok, err := store.CreateActionToken(42, "g1", 1001, uno.Action{Type: uno.DrawCard, PlayerID: 42}, 1*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	successCount := 0
	alreadyConsumedCount := 0
	var countMu sync.Mutex

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, status := store.ConsumeAction(tok, 42)
			countMu.Lock()
			defer countMu.Unlock()
			if status == ConsumeOK {
				successCount++
			} else if status == ConsumeAlreadyConsumed {
				alreadyConsumedCount++
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 ConsumeOK, got %d", successCount)
	}
	if alreadyConsumedCount != goroutines-1 {
		t.Fatalf("expected %d ConsumeAlreadyConsumed, got %d", goroutines-1, alreadyConsumedCount)
	}
}

func TestTokenStore_EvictionLimits(t *testing.T) {
	fakeNow := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	// global limit 3, user limit 2
	store := NewTokenStore(3, 2, func() time.Time { return fakeNow }, nil)

	// User 1 creates 2 tokens (reaches user limit)
	tok1, _ := store.CreateActionToken(1, "g1", 100, uno.Action{PlayerID: 1}, time.Hour)
	tok2, _ := store.CreateActionToken(1, "g1", 100, uno.Action{PlayerID: 1}, time.Hour)

	// User 1 creates 3rd token -> tok1 should be evicted by user limit
	tok3, _ := store.CreateActionToken(1, "g1", 100, uno.Action{PlayerID: 1}, time.Hour)

	_, status1 := store.ConsumeAction(tok1, 1)
	if status1 != ConsumeNotFound {
		t.Errorf("expected tok1 to be evicted (ConsumeNotFound), got %v", status1)
	}
	_, status2 := store.ConsumeAction(tok2, 1)
	if status2 != ConsumeOK {
		t.Errorf("expected tok2 to be valid, got %v", status2)
	}
	_, status3 := store.ConsumeAction(tok3, 1)
	if status3 != ConsumeOK {
		t.Errorf("expected tok3 to be valid, got %v", status3)
	}

	// Now user 2 creates tokens. Store currently has tok2 and tok3 (2 items, limit is 3)
	tok4, _ := store.CreateActionToken(2, "g1", 100, uno.Action{PlayerID: 2}, time.Hour) // 3 items
	tok5, _ := store.CreateActionToken(2, "g1", 100, uno.Action{PlayerID: 2}, time.Hour) // 4 items -> oldest global (tok2) evicted!

	_, statusTok2 := store.ConsumeAction(tok2, 1)
	if statusTok2 != ConsumeNotFound {
		t.Errorf("expected tok2 to be evicted by global limit, got %v", statusTok2)
	}
	_, statusTok4 := store.ConsumeAction(tok4, 2)
	if statusTok4 != ConsumeOK {
		t.Errorf("expected tok4 to be valid, got %v", statusTok4)
	}
	_, statusTok5 := store.ConsumeAction(tok5, 2)
	if statusTok5 != ConsumeOK {
		t.Errorf("expected tok5 to be valid, got %v", statusTok5)
	}
}

func TestTokenStore_Invalidation(t *testing.T) {
	store := NewTokenStore(100, 10, time.Now, nil)

	tokA, _ := store.CreateActionToken(1, "gameA", 10, uno.Action{PlayerID: 1}, time.Hour)
	tokB, _ := store.CreateActionToken(2, "gameA", 10, uno.Action{PlayerID: 2}, time.Hour)
	tokC, _ := store.CreateActionToken(1, "gameB", 20, uno.Action{PlayerID: 1}, time.Hour)

	// Invalidate gameA
	store.InvalidateGame("gameA")

	_, sA := store.ConsumeAction(tokA, 1)
	if sA != ConsumeNotFound {
		t.Errorf("expected tokA to be invalidated")
	}
	_, sB := store.ConsumeAction(tokB, 2)
	if sB != ConsumeNotFound {
		t.Errorf("expected tokB to be invalidated")
	}
	_, sC := store.ConsumeAction(tokC, 1)
	if sC != ConsumeOK {
		t.Errorf("expected tokC to remain valid")
	}
}

func TestTokenStore_CursorToken(t *testing.T) {
	fakeNow := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	store := NewTokenStore(100, 10, func() time.Time { return fakeNow }, nil)

	tok, err := store.CreateCursorToken(42, CursorKindHand, "g1", 5, 45, 2*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to consume cursor as action -> should fail
	_, status := store.ConsumeAction(tok, 42)
	if status != ConsumeInvalidType {
		t.Fatalf("expected ConsumeInvalidType, got %v", status)
	}

	// Get cursor with correct user
	cursor, err := store.GetCursor(tok, 42)
	if err != nil {
		t.Fatalf("expected get cursor to succeed, got %v", err)
	}
	if cursor.Kind != CursorKindHand || cursor.Offset != 45 || cursor.Revision != 5 {
		t.Errorf("unexpected cursor data: %+v", cursor)
	}

	// Get cursor with wrong user
	_, errWrongUser := store.GetCursor(tok, 99)
	if errWrongUser == nil {
		t.Errorf("expected error for wrong user")
	}

	// Expire cursor
	fakeNow = fakeNow.Add(5 * time.Minute)
	_, errExpired := store.GetCursor(tok, 42)
	if errExpired == nil {
		t.Errorf("expected error for expired cursor")
	}
}
