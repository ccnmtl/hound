package main // import "github.com/ccnmtl/hound"

import (
	"context"
	"encoding/json"
	"expvar"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/kelseyhightower/envconfig"
)

var (
	graphiteBase   string
	carbonBase     string
	metricBase     string
	emailFrom      string
	emailTo        string
	checkInterval  int
	globalThrottle int
	globalBackoff  int
	lastErrorEmail time.Time
	emailOnError   bool
	smtpServer     string
	smtpPort       int
	smtpUser       string
	smtpPassword   string
	window         string
)

var (
	expFailed         = expvar.NewInt("failed")
	expPassed         = expvar.NewInt("passed")
	expErrors         = expvar.NewInt("errors")
	expGlobalThrottle = expvar.NewInt("throttle")
	expGlobalBackoff  = expvar.NewInt("backoff")
	expUptime         = expvar.NewInt("uptime")
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

	f := configData{}
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
	graphiteBase = c.GraphiteBase
	carbonBase = c.CarbonBase
	metricBase = c.MetricBase
	emailFrom = c.EmailFrom
	emailTo = c.EmailTo
	checkInterval = c.CheckInterval
	globalThrottle = c.GlobalThrottle
	globalBackoff = 0
	emailOnError = c.EmailOnError
	smtpServer = c.SMTPServer
	smtpPort = c.SMTPPort
	smtpUser = c.SMTPUser
	smtpPassword = c.SMTPPassword
	window = c.Window

	// some defaults
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 5
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10
	}
	if window == "" {
		window = "10mins"
	}

	lastErrorEmail = time.Now()

	go func() {
		// update uptime
		for {
			time.Sleep(1 * time.Second)
			expUptime.Add(1)
		}
	}()

	bgcontext := context.Background()
	s, alertscancel := startServices(bgcontext, f, c)

	// wait for a SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// then gracefully shut everything down.
	alertscancel()

	// giving the http server 1 second to close its connections
	ctx, cancel := context.WithTimeout(bgcontext, 1*time.Second)
	defer cancel()

	if err = s.Shutdown(ctx); err != nil {
		log.WithFields(
			log.Fields{
				"error": fmt.Sprintf("%v", err),
			}).Fatal("graceful shutdown failed")
	} else {
		log.Info("successful graceful shutdown")
	}
}

func startServices(ctx context.Context, f configData, c config) (*http.Server, context.CancelFunc) {
	// initialize all the alerts
	ac := newAlertsCollection(smtpEmailer{})
	for _, a := range f.Alerts {
		emailTo := a.EmailTo
		if emailTo == "" {
			emailTo = c.EmailTo
		}
		ac.addAlert(newAlert(a.Name, a.Metric, a.Type, a.Threshold, a.Direction, httpFetcher{}, emailTo, a.RunBookLink))
	}

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
			pr := ac.MakeindivPageResponse(stringIdx)

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

	alertsctx, alertscancel := context.WithCancel(ctx)

	// kick off alerts in the background
	go ac.Run(alertsctx)

	// and the http server in the background
	go func() {
		log.Fatal(s.ListenAndServe())
	}()

	return s, alertscancel
}
