package main

import (
	"fmt"
	"time"
)

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
	for _, a := range ac.Alerts {
		if a.Status == "OK" {
			if a.PreviousStatus != "OK" {
				// this one has recovered. need to send a message
				a.SendRecoveryMessage()
			}
			// everything is peachy
		} else {
			// this one is broken. if we're not in a backoff period
			// we need to send a message
			if a.Throttled() {
				// wait for the throttling to expire
				fmt.Println("throttling...")
			} else {
				if alerts_sent < GLOBAL_THROTTLE {
					a.SendAlert()
					a.Backoff = a.Backoff + 1
					a.LastAlerted = time.Now()
				}
				alerts_sent++
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
				"This probably indicates an infrastructure problem (network, graphite, etc)", alerts_sent,
				GLOBAL_THROTTLE))
	}
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
