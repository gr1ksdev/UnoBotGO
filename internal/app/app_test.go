package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type migrator struct {
	events *[]string
	fail   string
}

func (m migrator) Migrate(context.Context) error {
	*m.events = append(*m.events, "migrate")
	if m.fail == "migrate" {
		return errors.New("migration failed")
	}
	return nil
}
func (m migrator) VerifySchema(context.Context) error {
	*m.events = append(*m.events, "verify")
	if m.fail == "verify" {
		return errors.New("checksum failed")
	}
	return nil
}
func TestStartupFailClosed(t *testing.T) {
	for _, fail := range []string{"", "migrate", "verify"} {
		t.Run(fail, func(t *testing.T) {
			events := []string{}
			err := Initialize(t.Context(), migrator{&events, fail}, func(context.Context) error { events = append(events, "http", "bot"); return nil })
			want := []string{"migrate", "verify", "http", "bot"}
			if fail == "migrate" {
				want = want[:1]
			} else if fail == "verify" {
				want = want[:2]
			}
			if !reflect.DeepEqual(events, want) || (err != nil) != (fail != "") {
				t.Fatal(events, err)
			}
		})
	}
}
