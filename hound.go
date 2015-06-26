package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

var (
	GRAPHITE_BASE    string
	CARBON_BASE      string
	METRIC_BASE      string
	EMAIL_FROM       string
	EMAIL_TO         string
	CHECK_INTERVAL   int
	GLOBAL_THROTTLE  int
	GLOBAL_BACKOFF   int
	LAST_ERROR_EMAIL time.Time
	EMAIL_ON_ERROR   bool
	SMTP_SERVER      string
	SMTP_PORT        int
)

type config struct {
	GraphiteBase      string `envconfig:"GRAPHITE_BASE"`
	CarbonBase        string `envconfig:"CARBON_BASE"`
	MetricBase        string `envconfig:"METRIC_BASE"`
	EmailFrom         string `envconfig:"EMAIL_FROM"`
	EmailTo           string `envconfig:"EMAIL_TO"`
	CheckInterval     int    `envconfig:"CHECK_INTERVAL"`
	GlobalThrottle    int    `envconfig:"GLOBAL_THROTTLE"`
	HTTPPort          string `envconfig:"HTTP_PORT"`
	TemplateFile      string `envconfig:"TEMPLATE_FILE"`
	AlertTemplateFile string `envconfig:"ALERT_TEMPLATE_FILE"`
	EmailOnError      bool   `envconfig:"EMAIL_ON_ERROR"`
	SMTPServer        string `envconfig:"SMTP_SERVER"`
	SMTPPort          int    `envconfig:"SMTP_PORT"`
}

func main() {
	// read the config file
	var configfile string
	flag.StringVar(&configfile, "config", "./config.json", "JSON config file")
	flag.Parse()

	file, err := ioutil.ReadFile(configfile)
	if err != nil {
		log.Fatal(err)
	}

	f := ConfigData{}
	err = json.Unmarshal(file, &f)
	if err != nil {
		log.Fatal(err)
	}

	var c config
	err = envconfig.Process("hound", &c)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("running on", c.HTTPPort)
	// set global values
	GRAPHITE_BASE = c.GraphiteBase
	CARBON_BASE = c.CarbonBase
	METRIC_BASE = c.MetricBase
	EMAIL_FROM = c.EmailFrom
	EMAIL_TO = c.EmailTo
	CHECK_INTERVAL = c.CheckInterval
	GLOBAL_THROTTLE = c.GlobalThrottle
	GLOBAL_BACKOFF = 0
	EMAIL_ON_ERROR = c.EmailOnError
	SMTP_SERVER = c.SMTPServer
	SMTP_PORT = c.SMTPPort

	LAST_ERROR_EMAIL = time.Now()

	// initialize all the alerts
	ac := NewAlertsCollection(SMTPEmailer{})
	for _, a := range f.Alerts {
		email_to := a.EmailTo
		if email_to == "" {
			email_to = c.EmailTo
		}
		ac.AddAlert(NewAlert(a.Name, a.Metric, a.Threshold, a.Direction, HTTPFetcher{}, email_to))
	}

	// kick it off in the background
	go ac.Run()

	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			pr := ac.MakePageResponse(0)

			t, err := template.ParseFiles(c.TemplateFile)
			if err != nil {
				log.Println(fmt.Sprintf("%v", err))
			}
			t.Execute(w, pr)
		})

	http.HandleFunc("/alert/",
		func(w http.ResponseWriter, r *http.Request) {
			stringIdx := strings.Split(r.URL.String(), "/")[2]
			idx, _ := strconv.Atoi(stringIdx)
			pr := ac.MakePageResponse(idx)

			if c.AlertTemplateFile == "" {
				// default to same location as index.html
				c.AlertTemplateFile = strings.Replace(c.TemplateFile, "index", "alert", 1)
			}

			t, err := template.ParseFiles(c.AlertTemplateFile)
			if err != nil {
				log.Println(fmt.Sprintf("%v", err))
			}
			t.Execute(w, pr)
		})

	log.Fatal(http.ListenAndServe(":"+c.HTTPPort, nil))
}
