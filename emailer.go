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

func (e SMTPEmailer) Throttled(failures, globalThrottle int, emailTo string) {
	simpleSendMail(
		EmailFrom,
		emailTo,
		"[ALERT] Hound is throttled",
		fmt.Sprintf("%d metrics were not OK.\nHound stopped sending messages after %d.\n"+
			"This probably indicates an infrastructure problem (network, graphite, etc)", failures,
			globalThrottle))
}

func (e SMTPEmailer) RecoveryThrottled(recoveriesSent, globalThrottle int, emailTo string) {
	if !EmailOnError {
		return
	}
	simpleSendMail(
		EmailFrom,
		emailTo,
		"[ALERT] Hound is recovered",
		fmt.Sprintf("%d metrics recovered.\nHound stopped sending individual messages after %d.\n",
			recoveriesSent,
			globalThrottle))
}

func (e SMTPEmailer) EncounteredErrors(errors int, emailTo string) {
	if !EmailOnError {
		return
	}
	simpleSendMail(
		EmailFrom,
		emailTo,
		"[ERROR] Hound encountered errors",
		fmt.Sprintf("%d metrics had errors. If this is more than a couple, it usually "+
			"means that Graphite has fallen behind. It doesn't necessarily mean "+
			"that there are problems with the services, but it means that Hound "+
			"is temporarily blind wrt these metrics.", errors))
}
