package main

import (
	"bufio"
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	ggArgsPrefix = "// ggArgs"
)

//go:embed templates/*
var fs embed.FS

type TemplateVariables struct {
	ObjectName      string
	ObjectIdName    string
	ObjectIdType    string
	Module          string
	DBPackage       string
	LogPackage      string
	CrudPackage     string
	UtilsPackage    string
	HttputilPackage string
}

var (
	validObjectIdTypes = map[string]bool{
		"string": true,
		"uint64": true,
	}
)

func main() {
	app := &cli.App{
		Name:        "ggcode",
		HelpName:    "ggcode",
		Description: "generate golang code for crud",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "objectName",
				Usage:    "object name, required",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "objectIdName",
				Usage: "object id name",
			},
			&cli.StringFlag{
				Name:        "objectIdType",
				Usage:       "object id type, can be string or uint64",
				Value:       "string",
				DefaultText: "string",
			},
			&cli.StringFlag{
				Name:  "module",
				Usage: "project module",
			},
			&cli.StringFlag{
				Name:     "db-package",
				Usage:    "import db pkg",
				EnvVars:  []string{"DB_PACKAGE"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "log-package",
				Usage:    "import log pkg",
				EnvVars:  []string{"LOG_PACKAGE"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "crud-package",
				Usage:    "import crud pkg",
				EnvVars:  []string{"CRUD_PACKAGE"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "utils-package",
				Usage:    "import utils pkg",
				EnvVars:  []string{"UTILS_PACKAGE"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "httputil-package",
				Usage:    "import httputil pkg",
				EnvVars:  []string{"HTTPUTIL_PACKAGE"},
				Required: true,
			},
		},
		Action: run,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("app run failed: %v", err)
	}

	return
}

func run(ctx *cli.Context) (err error) {
	objectName := ctx.String("objectName")

	objectIdName := ctx.String("objectIdName")
	if len(objectIdName) == 0 {
		objectIdName = objectName + "Id"
	}
	objectIdType := ctx.String("objectIdType")
	module := ctx.String("module")
	if len(module) == 0 {
		module, err = getModuleFromGoMod()
		if err != nil {
			log.Errorf("get module failed: %v, please set it", err)
			return
		}
	}

	if !validObjectIdTypes[objectIdType] {
		return fmt.Errorf("invalid objectIdType: %s", objectIdType)
	}

	tmplVars := TemplateVariables{
		ObjectName:      objectName,
		ObjectIdName:    objectIdName,
		ObjectIdType:    objectIdType,
		Module:          module,
		DBPackage:       ctx.String("db-package"),
		LogPackage:      ctx.String("log-package"),
		CrudPackage:     ctx.String("crud-package"),
		UtilsPackage:    ctx.String("utils-package"),
		HttputilPackage: ctx.String("httputil-package"),
	}

	ff, err := fs.ReadDir("templates")
	if err != nil {
		log.Errorf("fs.ReadDir failed: %v", err)
		return
	}

	for _, f := range ff {
		err = genCode(f.Name(), tmplVars)
		if err != nil {
			log.Errorf("gen code with %s failed: %v", f.Name(), err)
			return err
		}
	}
	return
}

func getModuleFromGoMod() (string, error) {
	f, err := os.Open("go.mod")
	if err != nil {
		log.Errorf("os.Open failed: %v", err)
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "module") {
			ss := strings.Split(text, " ")
			if len(ss) != 2 {
				return "", errors.New("can't parse module")
			}
			return ss[1], nil
		}
	}
	return "", errors.New("can't find module")
}

func genCode(tmplName string, tmplVars interface{}) (err error) {
	tmplData, err := fs.ReadFile("templates/" + tmplName)
	if err != nil {
		log.Errorf("read tmpl %s failed: %v", tmplName, err)
		return
	}
	s := string(tmplData)
	funcMap := template.FuncMap{
		"toSnake":      strcase.ToSnake,
		"toCamel":      strcase.ToCamel,
		"toLowerCamel": strcase.ToLowerCamel,
		"toLower":      strings.ToLower,
	}
	t := template.Must(template.New(tmplName).Funcs(funcMap).Parse(s))
	codeBuf := bytes.NewBuffer(nil)
	err = t.Execute(codeBuf, tmplVars)
	if err != nil {
		log.Errorf("tmpl execute failed: %v", err)
		return
	}
	targetCode := codeBuf.String()
	ggArgs, err := getGGArgs(targetCode)
	if err != nil {
		log.Errorf("get gg args failed: %v", err)
		return
	}
	targetCode = cleanGGArgs(targetCode)

	targetPath, ok := ggArgs["targetPath"]
	if !ok {
		log.Errorf("%s: can't find gg args targetPath, please set it on template file header: \"%s targetPath: path/xxx.go\"", tmplName, ggArgsPrefix)
		return
	}
	err = writeCodeToFile(targetPath, targetCode)
	if err != nil {
		log.Errorf("writeCodeToFile failed: %v", err)
		return
	}
	return
}

func getGGArgs(text string) (map[string]string, error) {
	m := make(map[string]string)
	ss := strings.Split(text, "\n")
	for i, s := range ss {
		if !strings.HasPrefix(s, ggArgsPrefix) {
			continue
		}
		s2 := strings.TrimLeft(s, ggArgsPrefix)
		kv := strings.Split(s2, ":")
		if len(kv) != 2 {
			return nil, fmt.Errorf("line: %d, invalid ggArgs: %s", i, s)
		}
		m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return m, nil
}

func cleanGGArgs(text string) string {
	ret := ""
	ss := strings.Split(text, "\n")
	for _, s := range ss {
		if strings.HasPrefix(s, ggArgsPrefix) {
			continue
		}
		ret += s + "\n"
	}
	return ret
}

func writeCodeToFile(path string, code string) (err error) {
	targetDir := filepath.Dir(path)
	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		log.Errorf("create target dir failed: %v", err)
		return
	}

	err = os.WriteFile(path, []byte(code), 0644)
	if err != nil {
		log.Errorf("write code to %s failed: %v", path, code)
		return
	}
	return
}
