package main

import (
	"github.com/pinpt/azure/internal"
	"github.com/pinpt/agent/v4/runner"
)

// Integration is used to export the integration
var Integration internal.AzureIntegration

func main() {
	runner.Main(&Integration)

}
