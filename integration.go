package main

import (
	"github.com/pinpt/agent.next.azure/internal"
	"github.com/pinpt/agent.next/runner"
)

// Integration is used to export the integration
var Integration internal.AzureIntegration

func main() {
	runner.Main(&Integration)

}
