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

type alert struct {
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
	fetcher        fetcher
	EmailTo        string
	Value          float64
	RunBookLink    string
}

var graphWidth = 800
var dailyGraphHeight = 150
var weeklyGraphHeight = 75
var fgColor = "000000"
var dailyBgColor = "FFFFFF"
var dailyColorlist = "%23999999,%23006699"
var weeklyBgColor = "EEEEEE"
var weeklyColorlist = "%23cccccc,%236699cc"

func newAlert(name string, metric string, atype string, threshold float64,
	direction string, fetcher fetcher, emailTo string, runbookLink string) *alert {
	if atype == "" {
		atype = "Alert"
	}
	return &alert{Name: name, Type: atype,
		Metric: cleanMetric(metric), Threshold: threshold, Direction: direction,
		Backoff: 0, LastAlerted: time.Now(), Status: "OK", Message: "",
		PreviousStatus: "OK", fetcher: fetcher, EmailTo: emailTo,
		Value: 0.0, RunBookLink: runbookLink,
	}
}

func cleanMetric(metric string) string {
	re := regexp.MustCompile("[ \n\t\r]+")
	return re.ReplaceAllString(metric, "")
}

func (a alert) URL() string {
	return graphiteBase + "?target=keepLastValue(" + a.Metric + ")&format=raw&from=-" + window
}

func (a alert) DailyGraphURL() string {
	return graphiteBase + "?target=" +
		a.Metric + "&target=threshold(" +
		fmt.Sprintf("%f", a.Threshold) +
		")&width=" + fmt.Sprintf("%d", graphWidth) +
		"&height=" + fmt.Sprintf("%d", dailyGraphHeight) +
		"&bgcolor=" + dailyBgColor +
		"&fgcolor=" + fgColor + "&hideGrid=true&colorList=" +
		dailyColorlist + "&from=-24hours"
}

func (a alert) WeeklyGraphURL() string {
	return graphiteBase + "?target=" +
		a.Metric + "&target=threshold(" +
		fmt.Sprintf("%f", a.Threshold) +
		")&width=" + fmt.Sprintf("%d", graphWidth) +
		"&height=" + fmt.Sprintf("%d", weeklyGraphHeight) +
		"&hideGrid=true&hideLegend=true&graphOnly=true&hideAxes=true&bgcolor=" +
		weeklyBgColor + "&fgcolor=" + fgColor +
		"&hideGrid=true&colorList=" + weeklyColorlist + "&from=-7days"
}

type fetcher interface {
	Get(string) (*http.Response, error)
}

type httpFetcher struct{}

func (h httpFetcher) Get(url string) (*http.Response, error) {
	client := http.Client{ Timeout: time.Second * 2 }
	return client.Get(url)
}

func (a *alert) Fetch() (float64, error) {
	resp, err := a.fetcher.Get(a.URL())
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

	// Close the response
	resp.Body.Close()

	return lv, err
}

func (a *alert) CheckMetric() bool {
	lv, err := a.Fetch()
	if err != nil {
		return false
	}
	a.UpdateStatus(lv)
	return a.Status == "OK"

}

func (a *alert) UpdateStatus(lv float64) {
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

func (a alert) String() string {
	if a.Status == "OK" {
		return fmt.Sprintf("%s\t%s [%s]", a.Status, a.Name, a.Metric)
	}
	return fmt.Sprintf("%s\t%s [%s]: %s (%s)", a.Status, a.Name, a.Metric, a.Message, a.LastAlerted)
}

func (a alert) RenderDirection() string {
	if a.Status == "OK" {
		if a.Direction == "above" {
			return "<"
		}
		return ">"
	}
	if a.Direction == "above" {
		return ">"
	}
	return "<"

}

func (a alert) BootstrapStatus() string {
	if a.Status == "OK" {
		return "OK"
	}
	if a.Status == "Failed" {
		return "danger"
	}
	return "warning"
}

func (a alert) Icon() string {
	if a.Type == "Notice" {
		return "info"
	}
	return "warning"

}

func (a *alert) SendRecoveryMessage() {
	log.WithFields(
		log.Fields{
			"name": a.Name,
		},
	).Debug("sending Recovery Message")
	simpleSendMail(emailFrom,
		a.EmailTo,
		a.RecoveryEmailSubject(),
		a.RecoveryEmailBody())
}

func (a *alert) RecoveryEmailSubject() string {
	return fmt.Sprintf("[RECOVERED] %s", a.Name)
}

func (a *alert) RecoveryEmailBody() string {
	return fmt.Sprintf("%s [%s] has returned %s %f", a.Name, a.Metric, invertDirection(a.Direction), a.Threshold)
}

func invertDirection(d string) string {
	if d == "above" {
		return "below"
	}
	return "above"
}

func (a *alert) Throttled() bool {
	if a.Backoff == 0 {
		return false
	}
	d := backoffTime(a.Backoff)
	window := a.LastAlerted.Add(d)
	return time.Now().Before(window)
}

func (a *alert) SendAlert() {
	log.WithFields(
		log.Fields{
			"name": a.Name,
		},
	).Debug("Sending Alert")
	simpleSendMail(emailFrom,
		a.EmailTo,
		a.alertEmailSubject(),
		a.alertEmailBody())
}

func (a *alert) alertEmailSubject() string {
	if a.Type == "Alert" {
		return fmt.Sprintf("[ALERT] %s", a.Name)
	}
	return fmt.Sprintf("[NOTICE] %s", a.Name)
}

func (a *alert) IncludeRunBookLink() string {
	if a.RunBookLink == "" {
		return ""
	}
	return fmt.Sprintf("\n\nRunbook link:\n%s\n", a.RunBookLink)
}

func (a *alert) alertEmailBody() string {
	return fmt.Sprintf("%s [%s] has triggered an alert\nStatus:\t%s\nMessage:\t%s\n\nDaily Graph: <%s>\nWeekly Graph: <%s>%s\n",
		a.Name, a.Metric, a.Status, a.Message, a.DailyGraphURL(), a.WeeklyGraphURL(), a.IncludeRunBookLink())
}

// did this alert just return to a healthy state?
// returns 1 if just recovered, 0 otherwise
func (a *alert) JustRecovered() bool {
	return a.PreviousStatus == "Failed" || a.PreviousStatus == "Error"
}

func (a *alert) SendRecoveryMessageIfNeeded(recoveriesSent int) {
	if a.JustRecovered() && recoveriesSent < globalThrottle {
		a.SendRecoveryMessage()
	}
}

func (a *alert) UpdateState(recoveriesSent int) (int, int, int, int, int) {
	successes := 0
	errors := 0
	failures := 0
	alertsSent := 0

	if a.Status == "OK" {
		successes++
		a.SendRecoveryMessageIfNeeded(recoveriesSent)
		if a.JustRecovered() {
			recoveriesSent++
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
					"recoveriesSent": recoveriesSent,
				},
			).Debug("throttled")
		} else {
			if a.Status == "Failed" && alertsSent < globalThrottle {
				a.SendAlert()
				alertsSent++
			}
			a.Backoff = intmin(a.Backoff+1, len(backoffDurations))
			a.LastAlerted = time.Now()
		}
	}
	// cycle the previous status
	a.PreviousStatus = a.Status
	return successes, recoveriesSent, errors, failures, alertsSent
}

func (a alert) Hash() string {
	h := sha1.New()
	io.WriteString(h, fmt.Sprintf("metric: %s", a.Metric))
	io.WriteString(h, fmt.Sprintf("direction: %s", a.Direction))
	io.WriteString(h, fmt.Sprintf("threshold: %f", a.Threshold))
	io.WriteString(h, fmt.Sprintf("type: %s", a.Type))
	return fmt.Sprintf("%x", h.Sum(nil))[0:10]
}

func extractLastValue(rawResponse string) (float64, error) {
	// just take the most recent value
	parts := strings.Split(strings.Trim(rawResponse, "\n\t "), ",")
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
	s := fmt.Sprintf("%s:%d", smtpServer, smtpPort)
	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpServer)

	if smtpPort == 25 {
		err := smtp.SendMail(s, auth, from, []string{to}, []byte(message))
		if err != nil {
			log.WithFields(
				log.Fields{
					"error":       err,
					"mail server": s,
				},
			).Error("error sending mail")
		}
		return err
	}
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         smtpServer,
	}

	conn, err := tls.Dial("tcp", s, tlsconfig)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("tls.Dial failed")
		return err
	}

	c, err := smtp.NewClient(conn, smtpServer)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("smtp.NewClient failed")
		return err
	}

	// Auth
	if err = c.Auth(auth); err != nil {
		log.WithFields(
			log.Fields{
				"err":           err,
				"SMTP_USER":     smtpUser,
				"SMTP_PASSWORD": smtpPassword,
				"SMTP_SERVER":   smtpServer,
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

func encodeRFC2047(String string) string {
	// use mail's rfc2047 to encode any string
	addr := mail.Address{Name: String, Address: ""}
	return strings.Trim(addr.String(), " <>")
}

var backoffDurations = []time.Duration{
	time.Duration(5) * time.Minute,
	time.Duration(30) * time.Minute,
	time.Duration(1) * time.Hour,
	time.Duration(2) * time.Hour,
	time.Duration(4) * time.Hour,
	time.Duration(8) * time.Hour,
	time.Duration(24) * time.Hour,
}

func backoffTime(level int) time.Duration {
	return backoffDurations[level]
}
