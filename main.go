package main

import (
	"github.com/couchbaselabs/cloud-monitoring-tool/monitoring"
	"github.com/couchbaselabs/cloud-monitoring-tool/views/slackbot"
	"log"
)

func main() {
	ctx, err := monitoring.AnalyseAWS()

	if err != nil {
		log.Fatalf("Something went horribly wrong when analysing clouds: %s", err)
	}

	slackBot := &slackbot.CloudMonitoringSlackBot{GlobalCloudContext: ctx}

	err = slackBot.PostThreadedReport()

	if err != nil {
		log.Fatalf("Something went horribly wrong when posting to Slack: %s", err)
	}
}

