/*
Copyright © 2022 Dyego Alexandre Eugenio github@dyego.com.br

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package main contains the main function.
package main

import (
	"os"

	"github.com/dyegoe/awss/cmd"
	"github.com/dyegoe/awss/logger"
)

func main() {
	log := logger.NewLogger(os.Stdout, map[string]string{"pkg": "main"})

	if err := cmd.Initialize(); err != nil {
		log.Error("error initializing the awss command", err)
	}
}
