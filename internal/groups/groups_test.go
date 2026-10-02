package groups

import (
	"context"
	"errors"
	"testing"
)

type memoryRepo struct {
	c      Config
	writes int
}

func (r *memoryRepo) GetOrCreateGroupConfig(context.Context, int64) (Config, error) { return r.c, nil }
func (r *memoryRepo) SetDefaultGameMode(_ context.Context, _ int64, m Mode) (Config, error) {
	r.writes++
	r.c.DefaultGameMode = m
	return r.c, nil
}
func (r *memoryRepo) SetRankingSystem(_ context.Context, _ int64, s RankingSystem) (Config, error) {
	r.writes++
	r.c.RankingSystem = s
	return r.c, nil
}
func (r *memoryRepo) SetInstalledBy(_ context.Context, _ int64, installerID int64) (Config, error) {
	r.writes++
	r.c.InstalledByUserID = &installerID
	return r.c, nil
}
func (r *memoryRepo) ObserveGroupTitle(_ context.Context, _ int64, title string) error {
	r.writes++
	r.c.Title = title
	return nil
}
func (r *memoryRepo) SetRankingPrivate(_ context.Context, _ int64, private bool) (Config, error) {
	r.writes++
	r.c.RankingPrivate = private
	return r.c, nil
}
func (r *memoryRepo) ToggleRankingPrivate(_ context.Context, _ int64) (Config, error) {
	r.writes++
	r.c.RankingPrivate = !r.c.RankingPrivate
	return r.c, nil
}

func TestPermission(t *testing.T) {
	installer := int64(7)
	for _, tt := range []struct {
		name      string
		id        int64
		role      Membership
		lookupErr error
		allow     bool
	}{
		{"admin", 8, Membership{Admin: true, Member: true}, nil, true},
		{"installer member", 7, Membership{Member: true}, nil, true},
		{"installer left", 7, Membership{}, nil, false},
		{"ordinary member", 8, Membership{Member: true}, nil, false},
		{"failed lookup", 7, Membership{Member: true}, errors.New("offline"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &memoryRepo{c: Defaults(42)}
			r.c.InstalledByUserID = &installer
			s := Service{Repository: r, LookupMembership: func(context.Context, int64, int64) (Membership, error) { return tt.role, tt.lookupErr }}
			_, err := s.SetDefaultGameMode(t.Context(), 42, tt.id, Caseiro)
			if (err == nil) != tt.allow {
				t.Fatalf("allow %t error %v", tt.allow, err)
			}
			if !tt.allow && r.writes != 0 {
				t.Fatal("unauthorized write")
			}

			_, allowed, err := s.CanConfigureUser(t.Context(), 42, tt.id)
			if (err == nil && allowed) != tt.allow {
				t.Fatalf("canConfigureUser allow %t allowed %t err %v", tt.allow, allowed, err)
			}

			_, err = s.SetRankingSystem(t.Context(), 42, tt.id, Updated)
			if (err == nil) != tt.allow {
				t.Fatalf("ranking allow %t error %v", tt.allow, err)
			}
		})
	}
}

func TestRecordInstallation(t *testing.T) {
	r := &memoryRepo{c: Defaults(42)}
	s := Service{Repository: r}
	c, err := s.RecordInstallation(t.Context(), 42, 99)
	if err != nil {
		t.Fatal(err)
	}
	if c.InstalledByUserID == nil || *c.InstalledByUserID != 99 {
		t.Fatalf("unexpected installed by: %v", c.InstalledByUserID)
	}
}

func TestNewGroupsDefaultToUpdated(t *testing.T) {
	c := Defaults(123)
	if c.RankingSystem != Updated || c.DefaultGameMode != Classic || c.Revision != 1 || c.RankingPrivate != false {
		t.Fatalf("incorrect new group defaults: %+v", c)
	}
}

func TestRankingPrivateTogglePermissions(t *testing.T) {
	installer := int64(7)
	r := &memoryRepo{c: Defaults(42)}
	r.c.InstalledByUserID = &installer
	s := Service{Repository: r, LookupMembership: func(_ context.Context, _, userID int64) (Membership, error) {
		if userID == 8 {
			return Membership{Admin: true, Member: true}, nil
		}
		if userID == 7 {
			return Membership{Member: true}, nil
		}
		return Membership{Member: true}, nil
	}}

	// Ordinary member cannot toggle
	_, err := s.ToggleRankingPrivate(t.Context(), 42, 999)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for ordinary member, got %v", err)
	}

	// Admin can toggle
	cfg, err := s.ToggleRankingPrivate(t.Context(), 42, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.RankingPrivate {
		t.Fatalf("expected RankingPrivate true, got false")
	}

	// Installer member can toggle
	cfg, err = s.ToggleRankingPrivate(t.Context(), 42, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RankingPrivate {
		t.Fatalf("expected RankingPrivate false, got true")
	}
}
