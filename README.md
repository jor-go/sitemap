# Golang Sitemap Builder
[![GoDoc](https://godoc.org/github.com/jor-go/sitemap?status.svg)](https://godoc.org/github.com/jor-go/sitemap)

### Import
```golang
import "github.com/jor-go/sitemap"
```



### Usage
```golang

package main

import (
    "github.com/jor-go/sitemap"
    "time"
    "fmt"
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
```


### Output
```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
   <url>
      <loc>https://example.com/page-1</loc>
      <lastmod>2019-03-19</lastmod>
      <changefreq>daily</changefreq>
      <priority>0.5</priority>
   </url>
   <url>
      <loc>https://example.com/page-2</loc>
      <lastmod>2019-03-19</lastmod>
      <changefreq>daily</changefreq>
      <priority>0.5</priority>
   </url>
   <url>
      <loc>https://example.com/page-3</loc>
      <lastmod>2019-03-19</lastmod>
      <changefreq>daily</changefreq>
      <priority>0.5</priority>
   </url>
</urlset>
```

### Generate Sitemap as Bytes
```golang

bytes, err := mySitemap.Generate()
if err != nil {
    fmt.Println(err)
}

// use bytes
```