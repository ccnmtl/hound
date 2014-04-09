package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type Alert struct {
	Name           string
	Metric         string
	Threshold      float64
	Direction      string
	Backoff        int
	LastAlerted    time.Time
	Status         string
	Message        string
	PreviousStatus string
}

func NewAlert(name string, metric string, threshold float64,
	direction string) *Alert {
	return &Alert{name, metric, threshold, direction, 0, time.Now(), "OK", "", "OK"}
}

func (a Alert) Url() string {
	return GRAPHITE_BASE + "?target=keepLastValue(" + a.Metric + ")&format=raw&from=-10mins"
}

func (a Alert) DailyGraphUrl() string {
	return GRAPHITE_BASE + "?target=" + a.Metric + "&target=threshold(" + fmt.Sprintf("%f", a.Threshold) + ")&width=800&height=300&from=-24hours"
}

func (a Alert) WeeklyGraphUrl() string {
	return GRAPHITE_BASE + "?target=" + a.Metric + "&target=threshold(" + fmt.Sprintf("%f", a.Threshold) + ")&width=800&height=300&from=-7days"
}

func (a *Alert) Fetch() (float64, error) {
	resp, err := http.Get(a.Url())
	if err != nil {
		a.Status = "Error"
		a.Message = "graphite request failed"
		return 0.0, errors.New("graphite request failed")
	}
	if resp.Status != "200 OK" {
		a.Status = "Error"
		a.Message = "graphite did not return 200 OK"
		return 0.0, errors.New("graphite did not return 200 OK")
	}
	b, _ := ioutil.ReadAll(resp.Body)
	s := fmt.Sprintf("%s", b)
	lv, err := extractLastValue(s)
	if err != nil {
		a.Status = "Error"
		a.Message = err.Error()
	}
	return lv, err
}

func (a *Alert) CheckMetric() bool {
	lv, err := a.Fetch()
	if err != nil {
		return false
	}
	if a.Direction == "above" {
		// pass if metric is below the threshold
		if lv < a.Threshold {
			a.Status = "OK"
			a.Message = ""
		} else {
			a.Status = "Failed"
			a.Message = fmt.Sprintf("%f >= %f", lv, a.Threshold)
		}
	} else {
		// pass if metric is above threshold
		if lv > a.Threshold {
			a.Status = "OK"
			a.Message = ""
		} else {
			a.Status = "Failed"
			a.Message = fmt.Sprintf("%f <= %f", lv, a.Threshold)
		}
	}
	return a.Status == "OK"
}

func (a Alert) String() string {
	if a.Status == "OK" {
		return fmt.Sprintf("%s\t%s [%s]", a.Status, a.Name, a.Metric)
	} else {
		return fmt.Sprintf("%s\t%s [%s]: %s (%s)", a.Status, a.Name, a.Metric, a.Message, a.LastAlerted)
	}
}

func (a *Alert) SendRecoveryMessage() {
	fmt.Printf("Sending Recovery Message for %s\n", a.Name)
	simpleSendMail(EMAIL_FROM,
		EMAIL_TO,
		a.RecoveryEmailSubject(),
		a.RecoveryEmailBody())
}

func (a *Alert) RecoveryEmailSubject() string {
	return fmt.Sprintf("[RECOVERED] %s", a.Name)
}

func (a *Alert) RecoveryEmailBody() string {
	return fmt.Sprintf("%s [%s] has returned %s %f", a.Name, a.Metric, invertDirection(a.Direction), a.Threshold)
}

func invertDirection(d string) string {
	if d == "above" {
		return "below"
	} else {
		return "above"
	}
}

func (a *Alert) Throttled() bool {
	d := backoff_time(a.Backoff)
	window := a.LastAlerted.Add(d)
	return time.Now().Before(window)
}

func (a *Alert) SendAlert() {
	fmt.Printf("Sending Alert for %s\n", a.Name)
	simpleSendMail(EMAIL_FROM,
		EMAIL_TO,
		a.AlertEmailSubject(),
		a.AlertEmailBody())
}

func (a *Alert) AlertEmailSubject() string {
	return fmt.Sprintf("[ALERT] %s", a.Name)
}

func (a *Alert) AlertEmailBody() string {
	return fmt.Sprintf("%s [%s] has triggered an alert\nStatus:\t%s\nMessage:\t%s\n\nDaily Graph: %s\nWeekly Graph: %s\n",
		a.Name, a.Metric, a.Status, a.Message, a.DailyGraphUrl(), a.WeeklyGraphUrl())
}

func extractLastValue(raw_response string) (float64, error) {
	// just take the most recent value
	parts := strings.Split(strings.Trim(raw_response, "\n\t "), ",")
	if len(parts) < 1 {
		return 0.0, errors.New("couldn't parse response")
	}
	return strconv.ParseFloat(parts[len(parts)-1], 64)
}

func simpleSendMail(from, to, subject string, body string) error {
	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	auth := smtp.PlainAuth("", "", "", "")
	return SendMail("127.0.0.1:25", auth, from, []string{to}, []byte(message))
}

func encodeRFC2047(String string) string {
	// use mail's rfc2047 to encode any string
	addr := mail.Address{String, ""}
	return strings.Trim(addr.String(), " <>")
}

func backoff_time(level int) time.Duration {
	backoff_durations := []time.Duration{
		time.Duration(5) * time.Minute,
		time.Duration(30) * time.Minute,
		time.Duration(1) * time.Hour,
		time.Duration(2) * time.Hour,
		time.Duration(4) * time.Hour,
		time.Duration(8) * time.Hour,
		time.Duration(24) * time.Hour,
	}
	return backoff_durations[level]
}
