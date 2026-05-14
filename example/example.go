package main

import (
	"log"
	"time"

	"github.com/jor-go/sitemap"
)

func main() {
	var sm sitemap.Sitemap
	siteLinks := []string{
		"https://example.com/page-1",
		"https://example.com/page-2",
		"https://example.com/page-3",
	}

	for _, link := range siteLinks {
		var u sitemap.URL
		if err := u.New(link, "daily", 0.5, time.Now()); err != nil {
			log.Fatal(err)
		}
		sm.AddURL(u)
	}

	if err := sm.GenerateAndSave("/tmp/sitemap.xml"); err != nil {
		log.Fatal(err)
	}
}
