package main

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

type PageResponse struct {
	GraphiteBase string
	Alerts       []*Alert
}

type AlertsCollection struct {
	Alerts  []*Alert
	Emailer Emailer
}

func NewAlertsCollection(e Emailer) *AlertsCollection {
	return &AlertsCollection{Emailer: e}
}

func (ac *AlertsCollection) AddAlert(a *Alert) {
	ac.Alerts = append(ac.Alerts, a)
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
	recoveries_sent := 0
	errors := 0
	failures := 0
	errored_alerts := make([]*Alert, 0)
	successes := 0

	for _, a := range ac.Alerts {
		if a.Status == "OK" {
			successes++
			recoveries_sent = recoveries_sent + a.StateOK(recoveries_sent)
		} else {
			// this one is broken. if we're not in a backoff period
			// we need to send a message
			if a.Status == "Error" {
				errors++
				errored_alerts = append(errored_alerts, a)
			} else {
				failures++
			}
			if a.Throttled() {
				// wait for the throttling to expire
			} else {
				if a.Status == "Failed" && alerts_sent < GLOBAL_THROTTLE {
					a.SendAlert()
					alerts_sent++
				}
				a.Backoff = intmin(a.Backoff+1, len(BACKOFF_DURATIONS))
				a.LastAlerted = time.Now()
			}
		}
		// cycle the previous status
		a.PreviousStatus = a.Status
	}
	if alerts_sent >= GLOBAL_THROTTLE {
		ac.Emailer.Throttled(failures, GLOBAL_THROTTLE)
	}

	if recoveries_sent >= GLOBAL_THROTTLE {
		ac.Emailer.RecoveryThrottled(recoveries_sent, GLOBAL_THROTTLE)
	}
	ac.HandleErrors(errors)
	LogToGraphite(alerts_sent, recoveries_sent, failures, errors, successes)
}

func (ac *AlertsCollection) HandleErrors(errors int) {
	if errors > 0 {
		d := backoff_time(GLOBAL_BACKOFF)
		window := LAST_ERROR_EMAIL.Add(d)
		if time.Now().After(window) {
			ac.Emailer.EncounteredErrors(errors)
			LAST_ERROR_EMAIL = time.Now()
			GLOBAL_BACKOFF = intmin(GLOBAL_BACKOFF+1, len(BACKOFF_DURATIONS))
		}
	} else {
		GLOBAL_BACKOFF = 0
	}
}

func LogToGraphite(alerts_sent, recoveries_sent, failures, errors, successes int) {
	var clientGraphite net.Conn
	clientGraphite, err := net.Dial("tcp", CARBON_BASE)
	if err != nil || clientGraphite == nil {
		return
	}
	defer clientGraphite.Close()
	now := int32(time.Now().Unix())
	buffer := bytes.NewBufferString("")

	fmt.Fprintf(buffer, "%salerts_sent %d %d\n", METRIC_BASE, alerts_sent, now)
	fmt.Fprintf(buffer, "%srecoveries_sent %d %d\n", METRIC_BASE, recoveries_sent, now)
	fmt.Fprintf(buffer, "%sfailures %d %d\n", METRIC_BASE, failures, now)
	fmt.Fprintf(buffer, "%serrors %d %d\n", METRIC_BASE, errors, now)
	fmt.Fprintf(buffer, "%ssuccesses %d %d\n", METRIC_BASE, successes, now)
	fmt.Fprintf(buffer, "%sglobal_backoff %d %d\n", METRIC_BASE, GLOBAL_BACKOFF, now)
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
		time.Sleep(time.Duration(CHECK_INTERVAL) * time.Minute)
	}
}

func (ac *AlertsCollection) DisplayAll() {
	for _, a := range ac.Alerts {
		fmt.Println(a)
	}
}

func (ac *AlertsCollection) MakePageResponse() PageResponse {
	pr := PageResponse{GraphiteBase: GRAPHITE_BASE}
	for _, a := range ac.Alerts {
		pr.Alerts = append(pr.Alerts, a)
	}
	return pr
}
