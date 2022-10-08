package main

import "github.com/dyegoe/awss/cmd"

func main() {
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
	var a = 1
}
