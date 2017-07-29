package main

import (
	"bytes"
	"fmt"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
)

type PageResponse struct {
	GraphiteBase string
	MetricBase   string
	Alerts       []*Alert
}

type IndivPageResponse struct {
	GraphiteBase string
	MetricBase   string
	Alert        *Alert
}

type AlertsCollection struct {
	Alerts       []*Alert
	AlertsByHash map[string]*Alert
	Emailer      Emailer
}

func NewAlertsCollection(e Emailer) *AlertsCollection {
	return &AlertsCollection{Emailer: e, AlertsByHash: make(map[string]*Alert)}
}

func (ac *AlertsCollection) AddAlert(a *Alert) {
	ac.Alerts = append(ac.Alerts, a)
	ac.AlertsByHash[a.Hash()] = a
}

func (ac *AlertsCollection) ByHash(s string) *Alert {
	return ac.AlertsByHash[s]
}

func (ac *AlertsCollection) CheckAll() {
	for _, a := range ac.Alerts {
		a.CheckMetric()
	}
}

func (ac *AlertsCollection) ProcessAll() {
	// fetch/calculate new status for all
	ac.CheckAll()
	alerts_sent := 0
	recoveriesSent := 0
	errors := 0
	failures := 0
	successes := 0

	for _, a := range ac.Alerts {
		s, rs, e, f, as := a.UpdateState(recoveriesSent)
		successes = successes + s
		recoveriesSent = recoveriesSent + rs
		errors = errors + e
		failures = failures + f
		alerts_sent = alerts_sent + as
	}
	if alerts_sent >= GlobalThrottle {
		ac.Emailer.Throttled(failures, GlobalThrottle, EmailTo)
	}

	if recoveriesSent >= GlobalThrottle {
		ac.Emailer.RecoveryThrottled(recoveriesSent, GlobalThrottle, EmailTo)
	}
	ac.HandleErrors(errors)
	LogToGraphite(alerts_sent, recoveriesSent, failures, errors, successes)
	ExposeVars(failures, errors, successes)
}

func ExposeVars(failures, errors, successes int) {
	ExpFailed.Set(int64(failures))
	ExpErrors.Set(int64(errors))
	ExpPassed.Set(int64(successes))
	ExpGlobalThrottle.Set(int64(GlobalThrottle))
	ExpGlobalBackoff.Set(int64(GlobalBackoff))
}

func (ac *AlertsCollection) HandleErrors(errors int) {
	if errors > 0 {
		d := backoff_time(GlobalBackoff)
		window := LastErrorEmail.Add(d)
		if time.Now().After(window) {
			ac.Emailer.EncounteredErrors(errors, EmailTo)
			LastErrorEmail = time.Now()
			GlobalBackoff = intmin(GlobalBackoff+1, len(BACKOFF_DURATIONS))
		}
	} else {
		GlobalBackoff = 0
	}
}

func LogToGraphite(alerts_sent, recoveriesSent, failures, errors, successes int) {
	var clientGraphite net.Conn
	clientGraphite, err := net.Dial("tcp", CarbonBase)
	if err != nil || clientGraphite == nil {
		return
	}
	defer clientGraphite.Close()
	now := int32(time.Now().Unix())
	buffer := bytes.NewBufferString("")

	fmt.Fprintf(buffer, "%salerts_sent %d %d\n", MetricBase, alerts_sent, now)
	fmt.Fprintf(buffer, "%srecoveries_sent %d %d\n", MetricBase, recoveriesSent, now)
	fmt.Fprintf(buffer, "%sfailures %d %d\n", MetricBase, failures, now)
	fmt.Fprintf(buffer, "%serrors %d %d\n", MetricBase, errors, now)
	fmt.Fprintf(buffer, "%ssuccesses %d %d\n", MetricBase, successes, now)
	fmt.Fprintf(buffer, "%sglobal_backoff %d %d\n", MetricBase, GlobalBackoff, now)
	clientGraphite.Write(buffer.Bytes())
}

func intmin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (ac *AlertsCollection) Run() {
	for {
		ac.ProcessAll()
		ac.DisplayAll()
		time.Sleep(time.Duration(CheckInterval) * time.Minute)
	}
}

func (ac *AlertsCollection) DisplayAll() {
	for _, a := range ac.Alerts {
		log.Debug(a)
	}
}

func (ac *AlertsCollection) MakePageResponse() PageResponse {
	pr := PageResponse{GraphiteBase: GraphiteBase,
		MetricBase: MetricBase}
	for _, a := range ac.Alerts {
		pr.Alerts = append(pr.Alerts, a)
	}
	return pr
}

func (ac *AlertsCollection) MakeIndivPageResponse(idx string) IndivPageResponse {
	return IndivPageResponse{GraphiteBase: GraphiteBase,
		MetricBase: MetricBase,
		Alert:      ac.ByHash(idx)}
}
