// Package main initializes the project and runs the query
package main

import (
	"github.com/xaaha/hulak/cmd"
)

func main() {
	// Assign the functions to the variables in the cmd package
	cmd.InitializeProject = InitializeProject
	cmd.HandleAPIRequests = HandleAPIRequests

	// Execute the root command
	cmd.Execute()
}
