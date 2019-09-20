package scraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gocolly/colly"
	"github.com/thedevsaddam/gojsonq"
)

type Node struct {
	Title    string `json:"toc_title"`
	Href     string `json:"href"`
	Children []Node `json:"children"`
}

// DownloadReference downloads and stores all of the Azure resources
// specification
func DownloadReference() error {
	referenceUrl := "http://docs.microsoft.com/en-us/azure/templates/toc.json"
	response, err := http.Get(referenceUrl)
	if err != nil {
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

	if err = referenceWriter(cleanBody); err != nil {
		return err
	}

	if err = referenceIterator(cleanBody); err != nil {
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

func referenceIterator(cleanBody []byte) error {
	var resources []Node
	if err := json.Unmarshal(cleanBody, &resources); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(resources))

	for _, v := range resources {
		go func(v Node) {
			defer wg.Done()
			for i, v := range v.Children {
				// We stop iterating after the most recent date (index 1)
				if i > 1 {
					break
				}
				if err := childIterator(v); err != nil {
					fmt.Println(err)
				}
			}
		}(v)
	}
	wg.Wait()

	return nil
}

func childIterator(v Node) error {
	for _, v := range v.Children {
		if v.Href == "" {
			childIterator(v)
			continue
		}
		if err := getAndSave(v.Href); err != nil {
			return err
		}
	}

	return nil
}

func getAndSave(refUrl string) error {
	res, url, err := getSpec(fmt.Sprintf("%s", refUrl))
	if err != nil {
		return err
	}
	if err := saveSpec(res, url); err != nil {
		return err
	}

	return nil
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

func saveSpec(spec string, url string) error {
	if spec == "" {
		return errors.New("specification can't be empty")
	}
	if url == "" {
		return errors.New("url can't be empty")
	}

	path := "https://docs.microsoft.com/en-us/azure/templates/"
	resource := url[strings.LastIndex(url, "/")+1:]

	var dir strings.Builder
	dir.WriteString("azure_templates/")
	dir.WriteString(strings.TrimSuffix(strings.TrimPrefix(url, path), resource))

	if _, err := os.Stat(dir.String()); os.IsNotExist(err) {
		if err := os.MkdirAll(dir.String(), os.ModePerm); err != nil {
			return err
		}
	}

	var file strings.Builder
	file.WriteString(dir.String() + resource + ".json")

	err := ioutil.WriteFile(file.String(), []byte(spec), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
