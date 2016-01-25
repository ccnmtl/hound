package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

type DummyFetcher struct{}

func (d DummyFetcher) Get(url string) (*http.Response, error) { return nil, nil }

func Test_String(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	if a.String() != "OK\tfoo [foo]" {
		t.Error("wrong value")
	}
	a.Status = "Failed"

	if !strings.HasPrefix(a.String(), "Failed") {
		t.Error(fmt.Sprintf("wrong value: %s", a.String()))
	}
}

func Test_Url(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	if a.Url() != "?target=keepLastValue(foo)&format=raw&from=-10mins" {
		t.Error(fmt.Sprintf("wrong value: %s", a.Url()))
	}
}

func Test_DailyGraphUrl(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	if a.DailyGraphUrl() != "?target=foo&target=threshold(10.000000)&width=800&height=150&bgcolor=FFFFFF&fgcolor=000000&hideGrid=true&colorList=%23999999,%23006699&from=-24hours" {
		t.Error(fmt.Sprintf("wrong value: %s", a.DailyGraphUrl()))
	}
}

func Test_WeeklyGraphUrl(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	if a.WeeklyGraphUrl() != "?target=foo&target=threshold(10.000000)&width=800&height=75&hideGrid=true&hideLegend=true&graphOnly=true&hideAxes=true&bgcolor=EEEEEE&fgcolor=000000&hideGrid=true&colorList=%23cccccc,%236699cc&from=-7days" {
		t.Error(fmt.Sprintf("wrong value: %s", a.WeeklyGraphUrl()))
	}
}

func Test_RecoveryEmailSubject(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	if a.RecoveryEmailSubject() != "[RECOVERED] foo" {
		t.Error(fmt.Sprintf("wrong value: %s", a.RecoveryEmailSubject()))
	}
}

func Test_AlertEmailSubject(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	if a.AlertEmailSubject() != "[ALERT] foo" {
		t.Error(fmt.Sprintf("wrong value: %s", a.AlertEmailSubject()))
	}
}

func Test_AlertEmailBody(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	if !strings.HasPrefix(a.AlertEmailBody(), "foo [foo] has triggered an alert") {
		t.Error(fmt.Sprintf("wrong value: %s", a.AlertEmailBody()))
	}
	a = NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "runbooklinkfoo")
	if !strings.Contains(a.AlertEmailBody(), "runbooklinkfoo") {
		t.Error(fmt.Sprintf("wrong value: %s", a.AlertEmailBody()))
	}
}

func Test_RecoveryEmailBody(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	if !strings.HasPrefix(a.RecoveryEmailBody(), "foo [foo] has returned below 10.000000") {
		t.Error(fmt.Sprintf("wrong value: %s", a.RecoveryEmailBody()))
	}
}

func Test_UpdateState(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	s, rs, e, f, as := a.UpdateState(0)
	if s != 1 {
		t.Error("s is wrong")
	}
	if rs != 0 {
		t.Error("rs is wrong")
	}
	if e != 0 {
		t.Error("e is wrong")
	}
	if f != 0 {
		t.Error("f is wrong")
	}
	if as != 0 {
		t.Error("as is wrong")
	}

}

func Test_extractLastValue(t *testing.T) {
	v, err := extractLastValue("1,2")
	if err != nil {
		t.Error("returned an error")
	}
	if v != 2.0 {
		t.Error("wrong value parsed")
	}
	v, err = extractLastValue("None")
	if err == nil {
		t.Error("should've returned an error")
	}
	if v != 0.0 {
		t.Error("should return 0")
	}
}

func Test_UpdateStatus(t *testing.T) {
	a := NewAlert("foo", "foo", 10, "above", DummyFetcher{}, "test@example.com", "")
	a.UpdateStatus(11.0)
	if a.Status != "Failed" {
		t.Error("should've failed")
	}
	a.UpdateStatus(9.0)
	if a.Status != "OK" {
		t.Error("should've passed")
	}
	a.Direction = "below"
	a.UpdateStatus(11.0)
	if a.Status != "OK" {
		t.Error("should've passed")
	}
	a.UpdateStatus(9.0)
	if a.Status != "Failed" {
		t.Error("should've failed")
	}
}
