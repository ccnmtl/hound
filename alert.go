package main

import (
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/mail"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

type Alert struct {
	Name           string
	Metric         string
	Type           string
	Threshold      float64
	Direction      string
	Backoff        int
	LastAlerted    time.Time
	Status         string
	Message        string
	PreviousStatus string
	Fetcher        Fetcher
	EmailTo        string
	Value          float64
	RunBookLink    string
}

var GRAPH_WIDTH = 800
var DAILY_GRAPH_HEIGHT = 150
var WEEKLY_GRAPH_HEIGHT = 75
var FGCOLOR = "000000"
var DAILY_BGCOLOR = "FFFFFF"
var DAILY_COLORLIST = "%23999999,%23006699"
var WEEKLY_BGCOLOR = "EEEEEE"
var WEEKLY_COLORLIST = "%23cccccc,%236699cc"

func NewAlert(name string, metric string, atype string, threshold float64,
	direction string, fetcher Fetcher, emailTo string, runbook_link string) *Alert {
	if atype == "" {
		atype = "Alert"
	}
	return &Alert{Name: name, Type: atype,
		Metric: cleanMetric(metric), Threshold: threshold, Direction: direction,
		Backoff: 0, LastAlerted: time.Now(), Status: "OK", Message: "",
		PreviousStatus: "OK", Fetcher: fetcher, EmailTo: emailTo,
		Value: 0.0, RunBookLink: runbook_link,
	}
}

func cleanMetric(metric string) string {
	re := regexp.MustCompile("[ \n\t\r]+")
	return re.ReplaceAllString(metric, "")
}

func (a Alert) Url() string {
	return GraphiteBase + "?target=keepLastValue(" + a.Metric + ")&format=raw&from=-" + WINDOW
}

func (a Alert) DailyGraphUrl() string {
	return GraphiteBase + "?target=" +
		a.Metric + "&target=threshold(" +
		fmt.Sprintf("%f", a.Threshold) +
		")&width=" + fmt.Sprintf("%d", GRAPH_WIDTH) +
		"&height=" + fmt.Sprintf("%d", DAILY_GRAPH_HEIGHT) +
		"&bgcolor=" + DAILY_BGCOLOR +
		"&fgcolor=" + FGCOLOR + "&hideGrid=true&colorList=" +
		DAILY_COLORLIST + "&from=-24hours"
}

func (a Alert) WeeklyGraphUrl() string {
	return GraphiteBase + "?target=" +
		a.Metric + "&target=threshold(" +
		fmt.Sprintf("%f", a.Threshold) +
		")&width=" + fmt.Sprintf("%d", GRAPH_WIDTH) +
		"&height=" + fmt.Sprintf("%d", WEEKLY_GRAPH_HEIGHT) +
		"&hideGrid=true&hideLegend=true&graphOnly=true&hideAxes=true&bgcolor=" +
		WEEKLY_BGCOLOR + "&fgcolor=" + FGCOLOR +
		"&hideGrid=true&colorList=" + WEEKLY_COLORLIST + "&from=-7days"
}

type Fetcher interface {
	Get(string) (*http.Response, error)
}

type HTTPFetcher struct{}

func (h HTTPFetcher) Get(url string) (*http.Response, error) {
	return http.Get(url)
}

func (a *Alert) Fetch() (float64, error) {
	resp, err := a.Fetcher.Get(a.Url())
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
	a.UpdateStatus(lv)
	return a.Status == "OK"

}

func (a *Alert) UpdateStatus(lv float64) {
	a.Value = lv
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
}

func (a Alert) String() string {
	if a.Status == "OK" {
		return fmt.Sprintf("%s\t%s [%s]", a.Status, a.Name, a.Metric)
	} else {
		return fmt.Sprintf("%s\t%s [%s]: %s (%s)", a.Status, a.Name, a.Metric, a.Message, a.LastAlerted)
	}
}

func (a Alert) RenderDirection() string {
	if a.Status == "OK" {
		if a.Direction == "above" {
			return "<"
		} else {
			return ">"
		}
	} else {
		if a.Direction == "above" {
			return ">"
		} else {
			return "<"
		}
	}
}

func (a Alert) BootstrapStatus() string {
	if a.Status == "OK" {
		return "OK"
	}
	if a.Status == "Failed" {
		return "danger"
	}
	return "warning"
}

func (a Alert) GlyphIcon() string {
	if a.Type == "Notice" {
		return "glyphicon-info-sign"
	} else {
		return "glyphicon-warning-sign"
	}
}

func (a *Alert) SendRecoveryMessage() {
	log.WithFields(
		log.Fields{
			"name": a.Name,
		},
	).Debug("sending Recovery Message")
	simpleSendMail(EmailFrom,
		a.EmailTo,
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
	if a.Backoff == 0 {
		return false
	}
	d := backoff_time(a.Backoff)
	window := a.LastAlerted.Add(d)
	return time.Now().Before(window)
}

func (a *Alert) SendAlert() {
	log.WithFields(
		log.Fields{
			"name": a.Name,
		},
	).Debug("Sending Alert")
	simpleSendMail(EmailFrom,
		a.EmailTo,
		a.AlertEmailSubject(),
		a.AlertEmailBody())
}

func (a *Alert) AlertEmailSubject() string {
	if a.Type == "Alert" {
		return fmt.Sprintf("[ALERT] %s", a.Name)
	} else {
		return fmt.Sprintf("[NOTICE] %s", a.Name)
	}
}

func (a *Alert) IncludeRunBookLink() string {
	if a.RunBookLink == "" {
		return ""
	}
	return fmt.Sprintf("\n\nRunbook link:\n%s\n", a.RunBookLink)
}

func (a *Alert) AlertEmailBody() string {
	return fmt.Sprintf("%s [%s] has triggered an alert\nStatus:\t%s\nMessage:\t%s\n\nDaily Graph: <%s>\nWeekly Graph: <%s>%s\n",
		a.Name, a.Metric, a.Status, a.Message, a.DailyGraphUrl(), a.WeeklyGraphUrl(), a.IncludeRunBookLink())
}

// did this alert just return to a healthy state?
// returns 1 if just recovered, 0 otherwise
func (a *Alert) JustRecovered() bool {
	return a.PreviousStatus == "Failed" || a.PreviousStatus == "Error"
}

func (a *Alert) SendRecoveryMessageIfNeeded(recoveries_sent int) {
	if a.JustRecovered() && recoveries_sent < GlobalThrottle {
		a.SendRecoveryMessage()
	}
}

func (a *Alert) UpdateState(recoveries_sent int) (int, int, int, int, int) {
	successes := 0
	errors := 0
	failures := 0
	alerts_sent := 0

	if a.Status == "OK" {
		successes++
		a.SendRecoveryMessageIfNeeded(recoveries_sent)
		if a.JustRecovered() {
			recoveries_sent++
		}
		a.Backoff = 0
	} else {
		// this one is broken. if we're not in a backoff period
		// we need to send a message
		if a.Status == "Error" {
			errors++
		} else {
			failures++
		}
		if a.Throttled() {
			// wait for the throttling to expire
			log.WithFields(
				log.Fields{
					"recoveries_sent": recoveries_sent,
				},
			).Debug("throttled")
		} else {
			if a.Status == "Failed" && alerts_sent < GlobalThrottle {
				a.SendAlert()
				alerts_sent++
			}
			a.Backoff = intmin(a.Backoff+1, len(BACKOFF_DURATIONS))
			a.LastAlerted = time.Now()
		}
	}
	// cycle the previous status
	a.PreviousStatus = a.Status
	return successes, recoveries_sent, errors, failures, alerts_sent
}

func (a Alert) Hash() string {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("metric: %s", a.Metric))
	io.WriteString(h, fmt.Sprintf("direction: %s", a.Direction))
	io.WriteString(h, fmt.Sprintf("threshold: %f", a.Threshold))
	io.WriteString(h, fmt.Sprintf("type: %s", a.Type))
	return fmt.Sprintf("%x", h.Sum(nil))[0:10]
}

func extractLastValue(raw_response string) (float64, error) {
	// just take the most recent value
	parts := strings.Split(strings.Trim(raw_response, "\n\t "), ",")
	return strconv.ParseFloat(parts[len(parts)-1], 64)
}

func simpleSendMail(from, to, subject string, body string) error {
	log.WithFields(
		log.Fields{
			"From":    from,
			"To":      to,
			"Subject": subject,
		},
	).Debug("simpleSendMail")
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
	s := fmt.Sprintf("%s:%d", SMTPServer, SMTP_PORT)
	auth := smtp.PlainAuth("", SMTP_USER, SMTP_PASSWORD, SMTPServer)

	if SMTP_PORT == 25 {
		err := SendMail(s, auth, from, []string{to}, []byte(message))
		if err != nil {
			log.WithFields(
				log.Fields{
					"error":       err,
					"mail server": s,
				},
			).Error("error sending mail")
		}
		return err
	} else {
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         SMTPServer,
		}

		conn, err := tls.Dial("tcp", s, tlsconfig)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("tls.Dial failed")
			return err
		}

		c, err := smtp.NewClient(conn, SMTPServer)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("smtp.NewClient failed")
			return err
		}

		// Auth
		if err = c.Auth(auth); err != nil {
			log.WithFields(
				log.Fields{
					"err":           err,
					"SMTP_USER":     SMTP_USER,
					"SMTP_PASSWORD": SMTP_PASSWORD,
					"SMTP_SERVER":   SMTPServer,
				}).Error("auth failed")
			return err
		}

		// To && From
		if err = c.Mail(from); err != nil {
			log.WithFields(log.Fields{"err": err, "from": from}).Error("from address failed")
			return err
		}

		if err = c.Rcpt(to); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("to address failed")
			return err
		}

		// Data
		w, err := c.Data()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("smtp Data() failed")
			return err
		}

		_, err = w.Write([]byte(message))
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("smtp Write failed")
			return err
		}

		err = w.Close()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("smtp close failed")
			return err
		}

		c.Quit()
		return err
	}

}

func encodeRFC2047(String string) string {
	// use mail's rfc2047 to encode any string
	addr := mail.Address{String, ""}
	return strings.Trim(addr.String(), " <>")
}

var BACKOFF_DURATIONS = []time.Duration{
	time.Duration(5) * time.Minute,
	time.Duration(30) * time.Minute,
	time.Duration(1) * time.Hour,
	time.Duration(2) * time.Hour,
	time.Duration(4) * time.Hour,
	time.Duration(8) * time.Hour,
	time.Duration(24) * time.Hour,
}

func backoff_time(level int) time.Duration {
	return BACKOFF_DURATIONS[level]
}
