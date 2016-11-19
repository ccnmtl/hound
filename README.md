[![Build Status](https://travis-ci.org/ccnmtl/hound.svg?branch=master)](https://travis-ci.org/ccnmtl/hound)

## Hound

This is a simple service that watches a number of Graphite metrics and
sends alert emails when they cross a threshold.

It automatically backs off on failing metrics. You'll get an email
when the metric first fails, another 5 minutes later, another 30
minutes after that, one hour after that, 2 hours after that, 4 hours,
8 hours, then every 24 hours thereafter. Finally, you will get an
email when the metric has recovered.

### Configuration

* `GraphiteBase`, `EmailFrom`, and `EmailTo` should all be obvious
* `CheckInterval` is how many minutes to wait between checks
* `GlobalThrottle` is the maximum number of alerts that Hound will
  send in a cycle. Ie, if there's a major network outage and all the
	metrics start failing, you want to stop it once you've figured that
  out. Once this threshold is passed, Hound sends just one more message
	saying how many metrics are failing.

Each Alert has:

* `Name`: obvious.
* `Type`: defaults to 'Alert'. You can also set it to 'Notice'. The
  convention is that an alert means "something is broken and a human
  needs to fix it NOW" while a Notice is just "you should know about
  this" and it's either not directly actionable or not urgent. Eg,
  high CPU load is probably a Notice, while high request error rates
  would warrant an alert.
* `Metric`: the actual Graphite metric being checked. This can be as
  complicated as you like and use the full suite of Graphite
  functions.
* `Threshold`: fairly obvious. Format it as a float. Treat it as ">="
  or "<=". Ie, it will trigger if the metric matches the threshold.
* `Direction`: "above" or "below". Specified whether a failure is when
  the metric crosses above or below the threshold, respectively.
