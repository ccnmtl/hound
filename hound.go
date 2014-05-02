package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var (
	GRAPHITE_BASE   string
	EMAIL_FROM      string
	EMAIL_TO        string
	CHECK_INTERVAL  int
	GLOBAL_THROTTLE int
	GLOBAL_BACKOFF int
	LAST_ERROR_EMAIL time.Time
)

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

	// set global values
	GRAPHITE_BASE = f.GraphiteBase
	EMAIL_FROM = f.EmailFrom
	EMAIL_TO = f.EmailTo
	CHECK_INTERVAL = f.CheckInterval
	GLOBAL_THROTTLE = f.GlobalThrottle
	GLOBAL_BACKOFF = 0
	LAST_ERROR_EMAIL = time.Now()

	// initialize all the alerts
	ac := NewAlertsCollection()
	for _, a := range f.Alerts {
		ac.AddAlert(NewAlert(a.Name, a.Metric, a.Threshold, a.Direction))
	}

	// kick it off in the background
	go ac.Run()

	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			pr := ac.MakePageResponse()

			t, err := template.ParseFiles(f.TemplateFile)
			if err != nil {
				fmt.Println(fmt.Sprintf("%v", err))
			}
			t.Execute(w, pr)
		})
	log.Fatal(http.ListenAndServe(":"+f.HttpPort, nil))
}
