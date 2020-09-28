package main

import (
	"github.com/pinpt/azure/internal"
	"github.com/pinpt/agent/runner"
)

// Integration is used to export the integration
var Integration internal.AzureIntegration

func main() {
	runner.Main(&Integration)

}
