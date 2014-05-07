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

func Test_emptyAlertsCollection(t *testing.T) {
	ac := NewAlertsCollection()
	ac.ProcessAll()
	ac.DisplayAll()
	ac.MakePageResponse()
}
