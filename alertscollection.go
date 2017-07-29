package main

import (
	"bytes"
	"fmt"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
)

type pageResponse struct {
	GraphiteBase string
	MetricBase   string
	Alerts       []*alert
}

type indivPageResponse struct {
	GraphiteBase string
	MetricBase   string
	Alert        *alert
}

type alertsCollection struct {
	alerts       []*alert
	alertsByHash map[string]*alert
	emailer      emailer
}

func newAlertsCollection(e emailer) *alertsCollection {
	return &alertsCollection{emailer: e, alertsByHash: make(map[string]*alert)}
}

func (ac *alertsCollection) addAlert(a *alert) {
	ac.alerts = append(ac.alerts, a)
	ac.alertsByHash[a.Hash()] = a
}

func (ac *alertsCollection) byHash(s string) *alert {
	return ac.alertsByHash[s]
}

func (ac *alertsCollection) checkAll() {
	for _, a := range ac.alerts {
		a.CheckMetric()
	}
}

func (ac *alertsCollection) processAll() {
	// fetch/calculate new status for all
	ac.checkAll()
	alertsSent := 0
	recoveriesSent := 0
	errors := 0
	failures := 0
	successes := 0

	for _, a := range ac.alerts {
		s, rs, e, f, as := a.UpdateState(recoveriesSent)
		successes = successes + s
		recoveriesSent = recoveriesSent + rs
		errors = errors + e
		failures = failures + f
		alertsSent = alertsSent + as
	}
	if alertsSent >= globalThrottle {
		ac.emailer.Throttled(failures, globalThrottle, emailTo)
	}

	if recoveriesSent >= globalThrottle {
		ac.emailer.RecoveryThrottled(recoveriesSent, globalThrottle, emailTo)
	}
	ac.handleErrors(errors)
	logToGraphite(alertsSent, recoveriesSent, failures, errors, successes)
	exposeVars(failures, errors, successes)
}

func exposeVars(failures, errors, successes int) {
	expFailed.Set(int64(failures))
	expErrors.Set(int64(errors))
	expPassed.Set(int64(successes))
	expGlobalThrottle.Set(int64(globalThrottle))
	expGlobalBackoff.Set(int64(globalBackoff))
}

func (ac *alertsCollection) handleErrors(errors int) {
	if errors > 0 {
		d := backoffTime(globalBackoff)
		window := lastErrorEmail.Add(d)
		if time.Now().After(window) {
			ac.emailer.EncounteredErrors(errors, emailTo)
			lastErrorEmail = time.Now()
			globalBackoff = intmin(globalBackoff+1, len(backoffDurations))
		}
	} else {
		globalBackoff = 0
	}
}

func logToGraphite(alertsSent, recoveriesSent, failures, errors, successes int) {
	var clientGraphite net.Conn
	clientGraphite, err := net.Dial("tcp", carbonBase)
	if err != nil || clientGraphite == nil {
		return
	}
	defer clientGraphite.Close()
	now := int32(time.Now().Unix())
	buffer := bytes.NewBufferString("")

	fmt.Fprintf(buffer, "%salerts_sent %d %d\n", metricBase, alertsSent, now)
	fmt.Fprintf(buffer, "%srecoveries_sent %d %d\n", metricBase, recoveriesSent, now)
	fmt.Fprintf(buffer, "%sfailures %d %d\n", metricBase, failures, now)
	fmt.Fprintf(buffer, "%serrors %d %d\n", metricBase, errors, now)
	fmt.Fprintf(buffer, "%ssuccesses %d %d\n", metricBase, successes, now)
	fmt.Fprintf(buffer, "%sglobal_backoff %d %d\n", metricBase, globalBackoff, now)
	clientGraphite.Write(buffer.Bytes())
}

func intmin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (ac *alertsCollection) Run() {
	for {
		ac.processAll()
		ac.DisplayAll()
		time.Sleep(time.Duration(checkInterval) * time.Minute)
	}
}

func (ac *alertsCollection) DisplayAll() {
	for _, a := range ac.alerts {
		log.Debug(a)
	}
}

func (ac *alertsCollection) MakePageResponse() pageResponse {
	pr := pageResponse{GraphiteBase: graphiteBase,
		MetricBase: metricBase}
	for _, a := range ac.alerts {
		pr.Alerts = append(pr.Alerts, a)
	}
	return pr
}

func (ac *alertsCollection) MakeindivPageResponse(idx string) indivPageResponse {
	return indivPageResponse{GraphiteBase: graphiteBase,
		MetricBase: metricBase,
		Alert:      ac.byHash(idx)}
}
