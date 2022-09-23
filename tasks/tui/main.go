package main

import (
	"flag"
	"fmt"

	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

func errmain() error {
	fmt.Println("Hello, world!")
	return nil
}

func main() {
	flag.Parse()
	if err := errmain(); err != nil {
		klog.Exit(err)
	}
}
