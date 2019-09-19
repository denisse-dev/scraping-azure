package scraper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/gocolly/colly"
	"github.com/thedevsaddam/gojsonq"
)

type Node struct {
	Title    string `json:"toc_title"`
	Href     string `json:"href"`
	Children []Node `json:"children"`
}

type Info struct {
	Title string
	Href  string
}

func DownloadReference() error {
	referenceUrl := "http://docs.microsoft.com/en-us/azure/templates/toc.json"
	response, err := http.Get(referenceUrl)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println(err)
		return err
	}

	cleanBody, err := referenceCleaner(body)
	if err != nil {
		return err
	}

	err = referenceWriter(cleanBody)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func referenceCleaner(body []byte) (cleanBody []byte, err error) {
	jsonQuery := gojsonq.New().JSONString(string(body))
	reference := jsonQuery.Find("items.[1].children")

	cleanBody, err = json.Marshal(reference)
	if err != nil {
		return nil, err
	}

	return cleanBody, nil
}

func referenceWriter(cleanBody []byte) error {
	filePath := "toc.json"

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = out.Write(cleanBody); err != nil {
		return err
	}

	return nil
}

func saveSpec(spec string, url string) {
	if spec == "" || url == "" {
		fmt.Println("Can't be!")
	}

	path := "https://docs.microsoft.com/en-us/azure/templates/"
	resource := url[strings.LastIndex(url, "/")+1:]

	var dir strings.Builder
	dir.WriteString("azure_templates/")
	dir.WriteString(strings.TrimSuffix(strings.TrimPrefix(url, path), resource))

	if _, err := os.Stat(dir.String()); os.IsNotExist(err) {
		if err := os.MkdirAll(dir.String(), os.ModePerm); err != nil {
			fmt.Println("We have a problem!")
		}
	}

	var file strings.Builder
	file.WriteString(dir.String() + resource + ".json")

	if err := ioutil.WriteFile(file.String(), []byte(spec), os.ModePerm); err != nil {
		panic(err)
	}
}

func getSpec(path string) (resource string, resURL string, err error) {
	var url strings.Builder
	url.WriteString("https://docs.microsoft.com/en-us/azure/templates/" + path)

	c := colly.NewCollector()

	c.OnHTML("code.lang-json", func(e *colly.HTMLElement) {
		resource = fmt.Sprintf("%v", *e)
		resource = strings.TrimPrefix(resource, "{code ")
		resource = resource[:strings.LastIndex(resource, "[{ class lang-json}]")]
	})

	if err := c.Visit(url.String()); err != nil {
		return "", "", err
	}

	return resource, url.String(), nil
}
