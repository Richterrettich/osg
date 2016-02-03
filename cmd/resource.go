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
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ResourceData struct {
	Attributes  map[string]string
	Name        string
	ProjectPath string
}

func (r ResourceData) BuildSelectAllStatement() string {
	stmt := "SELECT "
	for k, _ := range r.Attributes {
		stmt = stmt + strings.ToLower(k) + ","
	}
	return strings.TrimRight(stmt, ",") + " FROM " + r.Name + " \"+orderBy+\" LIMIT $1 SKIP $2"
}

func (r ResourceData) BuildSelectOneStatement() string {
	stmt := "SELECT "
	for k, _ := range r.Attributes {
		stmt = stmt + strings.ToLower(k) + ","
	}
	return strings.TrimRight(stmt, ",") + " FROM " + r.Name + " WHERE id = $1"
}

func (r ResourceData) BuildScanList() string {
	paramList := ""
	for k, _ := range r.Attributes {
		paramList = paramList + "&a." + k + ","
	}
	return strings.TrimRight(paramList, ",")
}

func (r ResourceData) BuildParameterListWithoutId() string {
	paramList := ""
	for k, v := range r.Attributes {
		paramList = fmt.Sprintf("%s %s %s,", paramList, strings.ToLower(k), v)
	}

	return strings.TrimRight(paramList, ",")
}

func (r ResourceData) BuildParameterList() string {
	paramList := "id string,"
	for k, v := range r.Attributes {
		paramList = fmt.Sprintf("%s %s %s,", paramList, strings.ToLower(k), v)
	}

	return strings.TrimRight(paramList, ",")
}

func (r ResourceData) BuildInsertStatementWithParameters() string {
	stmt := fmt.Sprintf("\"insert into %s(", r.Name)
	paramList := "values("
	realParameters := ""
	index := 1
	for k, _ := range r.Attributes {
		stmt = fmt.Sprintf("%s %s,", stmt, strings.ToLower(k))
		paramList = fmt.Sprintf("%s $%d,", paramList, index)
		realParameters = fmt.Sprintf("%s entity.%s,", realParameters, k)
		index = index + 1
	}
	return strings.TrimRight(stmt, ",") + ") " + strings.TrimRight(paramList, ",") + ")\",entity.Id," + strings.TrimRight(realParameters, ",")
}

func (r ResourceData) BuildUpdateStatementWithParameters() string {
	stmt := fmt.Sprintf("\"UPDATE %s SET ", r.Name)
	realParameters := ""
	index := 1
	for k, _ := range r.Attributes {
		stmt = fmt.Sprintf("%s %s=$%d,", stmt, strings.ToLower(k), index)
		realParameters = fmt.Sprintf("%s entity.%s,", realParameters, k)
		index = index + 1
	}
	return fmt.Sprintf("%s WHERE id = $%d \",%s,entity.Id", strings.TrimRight(stmt, ","), index, strings.TrimRight(realParameters, ","))
}

func (r ResourceData) LowercaseName() string {
	return strings.ToLower(r.Name)
}

const resourceHandlerTemplateString = `package handler

import (
	"{{.ProjectPath}}/mapper"
	"encoding/json"
	"net/http"

	"github.com/InteractiveLecture/middlewares/jwtware"
	"github.com/InteractiveLecture/paginator"
	"github.com/gorilla/mux"
)

func init() {
	s := Router.PathPrefix("/{{.LowercaseName}}/").Subrouter()

	s.Path("/").
		Methods("GET").
		Handler(jwtware.New(Get{{.Name}}PageHandler()))
	s.Path("/{id}").
		Methods("GET").
		Handler(jwtware.New(GetOne{{.Name}}Handler()))
	s.Path("/{id}").
		Methods("DELETE").
		Handler(jwtware.New(Delete{{.Name}}Handler()))
	s.Path("/").
		Methods("POST", "PUT").
		Handler(jwtware.New(CreateOrUpdate{{.Name}}Handler()))
}

func GetOne{{.Name}}Handler() http.Handler {

	result := func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		entity, err := mapper.GetOne{{.Name}}(vars["id"])
		if err != nil {
			w.WriteHeader(500)
		}
		if json.NewEncoder(w).Encode(entity) != nil {
			w.WriteHeader(500)
		}

	}
	return http.Handler(http.HandlerFunc(result))
}

func Get{{.Name}}PageHandler() http.Handler {
	result := func(w http.ResponseWriter, r *http.Request) {
		pr, err := paginator.ParsePages(r.URL)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		entity, err := mapper.Get{{.Name}}Page(pr)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		if json.NewEncoder(w).Encode(entity) != nil {
			w.WriteHeader(500)
		}
	}
	return http.Handler(http.HandlerFunc(result))
}

func Delete{{.Name}}Handler() http.Handler {
	result := func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]
		err := mapper.Delete{{.Name}}(id)
		if err != nil {
			w.WriteHeader(500)
			return
		}
	}
	return http.Handler(http.HandlerFunc(result))
}

func CreateOrUpdate{{.Name}}Handler() http.Handler {
	result := func(w http.ResponseWriter, r *http.Request) {
		entity := mapper.{{.Name}}{}
		err := json.NewDecoder(r.Body).Decode(&entity)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Method == "POST" {
			err = mapper.Create{{.Name}}(entity)
		} else {
			err = mapper.Update{{.Name}}(entity)
		}
		if err != nil {
			w.WriteHeader(500)
			return
		}
	}
	return http.Handler(http.HandlerFunc(result))
}
`

const resourceDatabaseTemplateString = `CREATE TABLE {{.Name}}(
	{{.BuildDatabaseFields}}
);
`

func (r ResourceData) BuildDatabaseFields() string {
	databaseFields := "id varchar PRIMARY KEY"
	for k, v := range r.Attributes {
		databaseFields = databaseFields + ",\n" + k
		switch v {
		case "int":
			databaseFields = databaseFields + " integer"
		case "string":
			databaseFields = databaseFields + " varchar"
		case "bool":
			databaseFields = databaseFields + " boolean"
		case "time.Time":
			databaseFields = databaseFields + " timestamp"
		default:
			fmt.Println("invalid datatype: ", v)
		}
	}
	return databaseFields
}

func (r ResourceData) CreateOrUpdateParameterList() string {
	paramList := "entity.Id"
	for k, _ := range r.Attributes {
		paramList = fmt.Sprintf("%s,entity.%s", paramList, k)
	}
	return paramList
}

const resourcePostgresMapperTemplateString = `package mapper
import (
	"fmt"
	"log"
	"strings"

	"github.com/InteractiveLecture/paginator"
)

type {{.Name}} struct {
	Id string
	{{range $key,$value := .Attributes}}
	{{$key}} {{$value}}
	{{end}}
}


func Get{{.Name}}Page(page paginator.PageRequest) ([]{{.Name}}, error) {
	orderBy := ""
	if len(page.Sorts) > 0 {
		orderBy = " ORDER BY "
		for _, v := range page.Sorts {
			orderBy = fmt.Sprintf("%s %s %v,", orderBy, v.Name, v.Direction)
		}
		orderBy = strings.TrimRight(orderBy, ",")
	}

	rows, err := dbmapper.Query("{{.BuildSelectAllStatement}}",page.Size,page.Number*page.Size)
	if err != nil {
		log.Println("error while retreiving page of {{.Name}}: ", err)
		return nil, err
	}
	defer rows.Close()
	result := make([]{{.Name}}, 0)
	for rows.Next() {
		a := {{.Name}}{}
		err = rows.Scan({{.BuildScanList}})
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return result, nil
}

func GetOne{{.Name}}(id string) (*{{.Name}}, error) {
	row := dbmapper.QueryRow("{{.BuildSelectOneStatement}}",id)
	a := {{.Name}}{}
	err := row.Scan({{.BuildScanList}})
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func Create{{.Name}}(entity {{.Name}}) error {
	return dbmapper.Execute({{.BuildInsertStatementWithParameters}})
}

func Update{{.Name}}(entity {{.Name}}) error {
	_, err := dbmapper.ExecuteRaw({{.BuildUpdateStatementWithParameters}})
	return err
}

func Delete{{.Name}}(id string) error {
	_, err := dbmapper.ExecuteRaw("DELETE FROM {{.Name}} where id = $1", id)
	return err
}
`

// resourceCmd represents the resource command
var resourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !viper.IsSet("project_path") {
			fmt.Println("you are either not within your project or you have a malformed .osg.yml. Can't proceed.")
			os.Exit(1)
		}
		rd := ResourceData{}
		rd.Attributes = make(map[string]string)
		rd.Name, args = strings.Title(args[0]), args[1:]
		database := viper.GetString("database")
		rd.ProjectPath = viper.GetString("project_path")

		for _, v := range args {
			parts := strings.Split(v, ":")
			if len(parts) != 2 {
				fmt.Println("Not a valid attribute-pair: ", v)
				os.Exit(1)
			}
			rd.Attributes[strings.Title(parts[0])] = parts[1]

		}
		fullPath := os.Getenv("GOPATH") + "/src/" + viper.GetString("project_path")
		log.Println(fullPath)
		switch database {
		case "postgres":
			createFileWithTemplate(fullPath+"/mapper/"+strings.ToLower(rd.Name)+"_mapper.go", rd, resourcePostgresMapperTemplateString)
			createFileWithTemplate(fullPath+"/handler/"+strings.ToLower(rd.Name)+"_handler.go", rd, resourceHandlerTemplateString)

			files, _ := ioutil.ReadDir(fullPath + "/database/ddl/")
			index := strconv.Itoa(len(files) + 1)
			createFileWithTemplate(fullPath+"/database/ddl/"+index+"_"+strings.ToLower(rd.Name)+".sql", rd, resourceDatabaseTemplateString)
		case "mongo":
		default:
		}
	},
}

func init() {
	createCmd.AddCommand(resourceCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// resourceCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// resourceCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}
