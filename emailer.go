package main

import (
	"fmt"
)

type Emailer interface {
	EncounteredErrors(int, string)
	RecoveryThrottled(int, int, string)
	Throttled(int, int, string)
}

type SMTPEmailer struct{}

func (e SMTPEmailer) Throttled(failures, global_throttle int, email_to string) {
	simpleSendMail(
		EMAIL_FROM,
		email_to,
		"[ALERT] Hound is throttled",
		fmt.Sprintf("%d metrics were not OK.\nHound stopped sending messages after %d.\n"+
			"This probably indicates an infrastructure problem (network, graphite, etc)", failures,
			global_throttle))
}

func (e SMTPEmailer) RecoveryThrottled(recoveries_sent, global_throttle int, email_to string) {
	if !EMAIL_ON_ERROR {
		return
	}
	simpleSendMail(
		EMAIL_FROM,
		email_to,
		"[ALERT] Hound is recovered",
		fmt.Sprintf("%d metrics recovered.\nHound stopped sending individual messages after %d.\n",
			recoveries_sent,
			global_throttle))
}

func (e SMTPEmailer) EncounteredErrors(errors int, email_to string) {
	if !EMAIL_ON_ERROR {
		return
	}
	simpleSendMail(
		EMAIL_FROM,
		email_to,
		"[ERROR] Hound encountered errors",
		fmt.Sprintf("%d metrics had errors. If this is more than a couple, it usually "+
			"means that Graphite has fallen behind. It doesn't necessarily mean "+
			"that there are problems with the services, but it means that Hound "+
			"is temporarily blind wrt these metrics.", errors))
}
