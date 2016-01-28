// Copyright © 2016 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

const mainTemplateString = `package main

import (
	"log"
	"net/http"

	"{{.ProjectPath}}/handler"
)

func main(){
	r := handler.Router
	log.Println("listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080",r))
}
`
const dockerfileTemplateString = `FROM alpine
ADD out/main /main
CMD ["/main"]
EXPOSE 8080
`

const makefileTemplateString = `
build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o out/main .

build_docker: build
	docker build -t openservice/{{.ServiceName}} .

integration_test: build_docker
	cd integration-test && docker-compose stop && docker-compose rm -f && docker-compose up -d && sleep 60 && go test; cd ..

push: integration_test
	docker login -u="$DOCKER_USERNAME" -p="$DOCKER_PASSWORD" -e="$DOCKER_EMAIL" \
	docker push openservice/lecture-service:latest
`

const handlerTemplateString = `package handler

import "github.com/gorilla/mux"

var Router *mux.Router = mux.NewRouter()
`

const dockerComposeTemplateString = `
version: 2
services:
	app:
		image: openservice/{{.ServiceName}}
		expose:
			- "8080"
		links:
			- "discovery:discovery"
			- "nats:nats"
			- "db:db"
		environment:
			- "SERVICE_NAME={{.ServiceName}}"
	{{if .HasDatabase}}
	db:
		image: {{.DbImage}}
		expose:
			- "{{.DbPort}}"
		environment:
			- "SERVICE_IGNORE=1"
			{{range $key, $value := .DbEnvs}}
			- "{{$key}}={{$value}}"
			{{end}}
	{{end}}
	discovery:
		image: progrium/consul 
		expose:
			- "8500"
			- "8400"
			- "53/udp"
	  command: "-server -bootstrap"

	registrator:
		image: gliderlabs/registrator
		volumes:
			- "var/run/docker.sock:/tmp/docker.sock"
		links:
			- "discovery:discovery"
		command: "-internal consul://discovery:8500"
	
	authentication:
		image: openservice/authentication-service
		links:
			- "authdatabase:postgres"
			- "discovery:discovery"
			- "nats:nats"
		environment:
			- "SERVICE_NAME=authentication-service"
	authdatabase:
		image: postgres
		expose:
			- "5432"
		environment:
			- "SERVICE_IGNORE=1"
	nats:
		image: nats
		expose:
			- "4222"
	natsremote:
		image: openservice/nats-remote
		expose: 
			- "8080"
		environment:
			- "SERVICE_NAME=nats-remote"
`

var database string

type InitConfig struct {
	ServiceName string
	DbImage     string
	DbPort      int
	DbEnvs      map[string]string
	ProjectPath string
	HasDatabase bool
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			log.Fatal("no names given.")
		}
		dockerFileTmpl := createTemplate("Dockerfile", dockerfileTemplateString)
		makefileTmpl := createTemplate("Makefile", makefileTemplateString)
		handlerTmpl := createTemplate("handler.go", handlerTemplateString)
		mainTmpl := createTemplate("main.go", mainTemplateString)
		dockerComposeTmpl := createTemplate("docker-compose.yml", dockerComposeTemplateString)

		for _, v := range args {
			parts := strings.Split(v, "/")
			serviceName := parts[len(parts)-1]
			con := InitConfig{}
			con.ServiceName = serviceName
			switch database {
			case "postgres":
				con.DbImage = "postgres"
				con.DbPort = 54322
				con.HasDatabase = true
				con.DbEnvs = map[string]string{
					"POSTGRES_PASSWORD": "postgres",
					"POSTGRES_USER":     "postgres",
				}
			case "mongo":
				con.DbImage = "mongo"
				con.DbPort = 27017
				con.HasDatabase = true
			case "nope":
				con.HasDatabase = false
			default:
				fmt.Print("unknown database: ", database)
				os.Exit(1)
			}

			con.ProjectPath = v

			fullPath := os.Getenv("GOPATH") + "/src/" + v

			CheckError(os.MkdirAll(fullPath+"/integration-test", os.ModePerm))
			CheckError(os.Mkdir(fullPath+"/handler", os.ModePerm))
			if database != "nope" {
				CheckError(os.Mkdir(fullPath+"/mapper", os.ModePerm))
			}
			if database == "postgres" {
				CheckError(os.MkdirAll(fullPath+"/database/ddl", os.ModePerm))
			}

			createFile(fullPath+"/main.go", con, mainTmpl)
			createFile(fullPath+"/Dockerfile", con, dockerFileTmpl)
			createFile(fullPath+"/integration-test/docker-compose.yml", con, dockerComposeTmpl)
			createFile(fullPath+"/handler/handler.go", con, handlerTmpl)
			createFile(fullPath+"/Makefile", con, makefileTmpl)
		}
	},
}

func createTemplate(name, content string) *template.Template {
	result := template.New(name)
	result, err := result.Parse(content)
	CheckError(err)
	return result
}

func createFile(path string, data interface{}, template *template.Template) {
	f, err := os.Create(path)
	CheckError(err)
	defer f.Close()
	CheckError(template.Execute(f, data))
}

func init() {
	RootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&database, "database", "d", "postgres", "the database backend")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// initCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}
