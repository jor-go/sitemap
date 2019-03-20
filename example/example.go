package main

import (
	"fmt"
	"time"

	"github.com/jor-go/sitemap"
)

func main() {
	var mySitemap sitemap.Sitemap
	siteLinks := []string{
		"https://example.com/page-1",
		"https://example.com/page-2",
		"https://example.com/page-3",
	}

	for _, link := range siteLinks {
		url := sitemap.URL{}
		err := url.New(link, "daily", 0.5, time.Now())
		if err != nil {
			fmt.Println(err)
			break
		}
		mySitemap.AddURL(url)
	}

	err := mySitemap.GenerateAndSave("/tmp/sitemap.xml")
	if err != nil {
		fmt.Println(err)
	}
}
