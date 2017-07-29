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
	ac := newAlertsCollection(DummyEmailer{})
	ac.processAll()
	ac.DisplayAll()
	ac.MakePageResponse()
}

func Test_handleErrors(t *testing.T) {
	ac := newAlertsCollection(DummyEmailer{})
	ac.handleErrors(1)
}

func Test_addAlert(t *testing.T) {
	ac := newAlertsCollection(DummyEmailer{})
	a := newAlert("foo", "foo", "", 10, "above", DummyFetcher{}, "test@example.com", "")
	ac.addAlert(a)

	ac.DisplayAll()
	ac.MakePageResponse()

	retrieved := ac.byHash(a.Hash())
	if retrieved != a {
		t.Error("failed to retrieve alert")
	}
}
