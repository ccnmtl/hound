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

func (d DummyEmailer) Throttled(failures, globalThrottle int, emailTo string)               {}
func (d DummyEmailer) RecoveryThrottled(recoveriesSent, globalThrottle int, emailTo string) {}
func (d DummyEmailer) EncounteredErrors(errors int, emailTo string)                         {}

func Test_emptyAlertsCollection(t *testing.T) {
	ac := NewAlertsCollection(DummyEmailer{})
	ac.ProcessAll()
	ac.DisplayAll()
	ac.MakePageResponse()
}

func Test_HandleErrors(t *testing.T) {
	ac := NewAlertsCollection(DummyEmailer{})
	ac.HandleErrors(1)
}

func Test_AddAlert(t *testing.T) {
	ac := NewAlertsCollection(DummyEmailer{})
	a := NewAlert("foo", "foo", "", 10, "above", DummyFetcher{}, "test@example.com", "")
	ac.AddAlert(a)

	ac.DisplayAll()
	ac.MakePageResponse()

	retrieved := ac.ByHash(a.Hash())
	if retrieved != a {
		t.Error("failed to retrieve alert")
	}
}
