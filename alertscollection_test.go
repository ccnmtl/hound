package main

import (
	"testing"
)

func Test_intmin(t *testing.T) {
	if intmin(1, 2) != 1 {
		t.Error("wrong")
	}
	if intmin(2, 1) != 1 {
		t.Error("wrong")
	}
}

type DummyEmailer struct{}

func (d DummyEmailer) Throttled(failures, global_throttle int, email_to string)                {}
func (d DummyEmailer) RecoveryThrottled(recoveries_sent, global_throttle int, email_to string) {}
func (d DummyEmailer) EncounteredErrors(errors int, email_to string)                           {}

func Test_emptyAlertsCollection(t *testing.T) {
	ac := NewAlertsCollection(DummyEmailer{})
	ac.ProcessAll()
	ac.DisplayAll()
	ac.MakePageResponse(0)
}

func Test_HandleErrors(t *testing.T) {
	ac := NewAlertsCollection(DummyEmailer{})
	ac.HandleErrors(1)
}

func Test_AddAlert(t *testing.T) {
	ac := NewAlertsCollection(DummyEmailer{})
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	ac.AddAlert(a)

	ac.DisplayAll()
	ac.MakePageResponse(0)
}
