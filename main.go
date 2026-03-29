package main

import (
	"fmt"
	"os"

	"github.com/ffreis/dynamoctl/cmd"
)

var exit = os.Exit

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(1)
	}
}
