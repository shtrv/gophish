package main

/*
gophish - Open-Source Phishing Framework

The MIT License (MIT)

Copyright (c) 2013 Jordan Wright

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"

	"github.com/shtrv/gophish/config"
	"github.com/shtrv/gophish/controllers"
	"github.com/shtrv/gophish/dialer"
	"github.com/shtrv/gophish/imap"
	log "github.com/shtrv/gophish/logger"
	"github.com/shtrv/gophish/middleware"
	"github.com/shtrv/gophish/models"
	"github.com/shtrv/gophish/webhook"
)

const (
	modeAll   = "all"
	modeAdmin = "admin"
	modePhish = "phish"
)

var (
	configPath    string
	disableMailer bool
	mode          string
)

func init() {
	flag.StringVar(&configPath, "config", "./config.json", "Location of config.json")
	flag.BoolVar(&disableMailer, "disable-mailer", false, "Disable the mailer (for use with multi-system deployments)")
	flag.StringVar(&mode, "mode", modeAll, fmt.Sprintf("Run the binary in one of the modes (%s, %s or %s)", modeAll, modeAdmin, modePhish))
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if mode != modeAll && mode != modeAdmin && mode != modePhish {
		fmt.Fprintf(os.Stderr, "Invalid mode: %s\n", mode)
		flag.Usage()
		os.Exit(1)
	}

	// Load the version
	version, err := ioutil.ReadFile("./VERSION")
	if err != nil {
		log.Fatal(err)
	}
	config.Version = string(version)

	// Load the config
	conf, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	if conf.ContactAddress == "" {
		log.Warnf("No contact address has been configured.")
		log.Warnf("Please consider adding a contact_address entry in your config.json")
	}

	// Configure upstream clients
	dialer.SetAllowedHosts(conf.AdminConf.AllowedInternalHosts)
	webhook.SetTransport(&http.Transport{
		DialContext: dialer.Dialer().DialContext,
	})

	err = log.Setup(conf.Logging)
	if err != nil {
		log.Fatal(err)
	}

	err = models.Setup(conf)
	if err != nil {
		log.Fatal(err)
	}
	err = models.UnlockAllMailLogs()
	if err != nil {
		log.Fatal(err)
	}

	// Setup servers
	adminOptions := []controllers.AdminServerOption{}
	if disableMailer {
		adminOptions = append(adminOptions, controllers.WithWorker(nil))
	}
	adminServer := controllers.NewAdminServer(conf.AdminConf, adminOptions...)
	middleware.Store.Options.Secure = conf.AdminConf.UseTLS

	phishServer := controllers.NewPhishingServer(conf.PhishConf)
	imapMonitor := imap.NewMonitor()

	if mode == modeAdmin || mode == modeAll {
		go adminServer.Start()
		go imapMonitor.Start()
	}
	if mode == modePhish || mode == modeAll {
		go phishServer.Start()
	}

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Info("CTRL+C Received... Gracefully shutting down servers")

	if mode == modeAdmin || mode == modeAll {
		adminServer.Shutdown()
		imapMonitor.Shutdown()
	}
	if mode == modePhish || mode == modeAll {
		phishServer.Shutdown()
	}
}
