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
		})
	}
}
