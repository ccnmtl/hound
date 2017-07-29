package main // import "github.com/ccnmtl/hound"

import (
	"encoding/json"
	"expvar"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/kelseyhightower/envconfig"
)

var (
	GraphiteBase     string
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
	SMTP_USER        string
	SMTP_PASSWORD    string
	WINDOW           string
)

var (
	EXP_FAILED          = expvar.NewInt("failed")
	EXP_PASSED          = expvar.NewInt("passed")
	EXP_ERRORS          = expvar.NewInt("errors")
	EXP_GLOBAL_THROTTLE = expvar.NewInt("throttle")
	EXP_GLOBAL_BACKOFF  = expvar.NewInt("backoff")
	EXP_UPTIME          = expvar.NewInt("uptime")
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
	SMTPUser          string `envconfig:"SMTP_USER"`
	SMTPPassword      string `envconfig:"SMTP_PASSWORD"`
	LogLevel          string `envconfig:"LOG_LEVEL"`
	ReadTimeout       int    `envconfig:"READ_TIMEOUT"`
	WriteTimeout      int    `envconfig:"WRITE_TIMEOUT"`
	Window            string `envconfig:"WINDOW"`
}

func main() {
	log.SetLevel(log.InfoLevel)
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

	// defaults to INFO
	if c.LogLevel == "DEBUG" {
		log.SetLevel(log.DebugLevel)
	}
	if c.LogLevel == "WARN" {
		log.SetLevel(log.WarnLevel)
	}
	if c.LogLevel == "ERROR" {
		log.SetLevel(log.ErrorLevel)
	}
	if c.LogLevel == "FATAL" {
		log.SetLevel(log.FatalLevel)
	}

	log.Info("running on ", c.HTTPPort)
	// set global values
	GraphiteBase = c.GraphiteBase
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
	SMTP_USER = c.SMTPUser
	SMTP_PASSWORD = c.SMTPPassword
	WINDOW = c.Window

	// some defaults
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 5
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10
	}
	if WINDOW == "" {
		WINDOW = "10mins"
	}

	LAST_ERROR_EMAIL = time.Now()

	go func() {
		// update uptime
		for {
			time.Sleep(1 * time.Second)
			EXP_UPTIME.Add(1)
		}
	}()

	// initialize all the alerts
	ac := NewAlertsCollection(SMTPEmailer{})
	for _, a := range f.Alerts {
		email_to := a.EmailTo
		if email_to == "" {
			email_to = c.EmailTo
		}
		ac.AddAlert(NewAlert(a.Name, a.Metric, a.Type, a.Threshold, a.Direction, HTTPFetcher{}, email_to, a.RunBookLink))
	}

	// kick it off in the background
	go ac.Run()

	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			pr := ac.MakePageResponse()

			t, err := template.ParseFiles(c.TemplateFile)
			if err != nil {
				log.WithFields(log.Fields{
					"error": fmt.Sprintf("%v", err),
				}).Fatal("Error parsing template")
			}
			t.Execute(w, pr)
		})

	http.HandleFunc("/alert/",
		func(w http.ResponseWriter, r *http.Request) {
			stringIdx := strings.Split(r.URL.String(), "/")[2]
			pr := ac.MakeIndivPageResponse(stringIdx)

			if c.AlertTemplateFile == "" {
				// default to same location as index.html
				c.AlertTemplateFile = strings.Replace(c.TemplateFile, "index", "alert", 1)
			}

			t, err := template.ParseFiles(c.AlertTemplateFile)
			if err != nil {
				log.Fatal(fmt.Sprintf("%v", err))
			}
			t.Execute(w, pr)
		})
	s := &http.Server{
		Addr:         ":" + c.HTTPPort,
		ReadTimeout:  time.Duration(c.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(c.WriteTimeout) * time.Second,
	}
	log.Fatal(s.ListenAndServe())
}
