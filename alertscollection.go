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
	Alerts []*Alert
}

func NewAlertsCollection() *AlertsCollection {
	return &AlertsCollection{}
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
			if a.PreviousStatus == "Failed" {
				// this one has recovered. need to send a message
				if recoveries_sent < GLOBAL_THROTTLE {
					a.SendRecoveryMessage()
				}
				recoveries_sent++
			}
			// everything is peachy
			a.Backoff = 0
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
				a.Backoff = intmin(a.Backoff + 1, len(BACKOFF_DURATIONS))
				a.LastAlerted = time.Now()
			}
		}
		// cycle the previous status
		a.PreviousStatus = a.Status
	}
	if alerts_sent >= GLOBAL_THROTTLE {
		simpleSendMail(
			EMAIL_FROM,
			EMAIL_TO,
			"[ALERT] Hound is throttled",
			fmt.Sprintf("%d metrics were not OK.\nHound stopped sending messages after %d.\n"+
				"This probably indicates an infrastructure problem (network, graphite, etc)", failures,
				GLOBAL_THROTTLE))
	}

	if recoveries_sent >= GLOBAL_THROTTLE {
		simpleSendMail(
			EMAIL_FROM,
			EMAIL_TO,
			"[ALERT] Hound is recovered",
			fmt.Sprintf("%d metrics recovered.\nHound stopped sending individual messages after %d.\n",
				recoveries_sent,
				GLOBAL_THROTTLE))
	}
	if errors > 0 {
		d := backoff_time(GLOBAL_BACKOFF)
		window := LAST_ERROR_EMAIL.Add(d)
		if time.Now().After(window) {
			simpleSendMail(
				EMAIL_FROM,
				EMAIL_TO,
				"[ERROR] Hound encountered errors",
				fmt.Sprintf("%d metrics had errors. If this is more than a couple, it usually "+
					"means that Graphite has fallen behind. It doesn't necessarily mean "+
					"that there are problems with the services, but it means that Hound "+
					"is temporarily blind wrt these metrics.", errors))
			LAST_ERROR_EMAIL = time.Now()
			GLOBAL_BACKOFF = intmin(GLOBAL_BACKOFF + 1, len(BACKOFF_DURATIONS))
		}
	} else {
		GLOBAL_BACKOFF = 0
	}
	LogToGraphite(alerts_sent, recoveries_sent, failures, errors, successes)
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
