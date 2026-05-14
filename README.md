# Sitemap

[![GoDoc](https://godoc.org/github.com/jor-go/sitemap?status.svg)](https://godoc.org/github.com/jor-go/sitemap)

Go library for generating XML sitemaps and sitemap index files using the
[Sitemaps XML protocol](https://www.sitemaps.org/protocol.html).

## Install

```sh
go get github.com/jor-go/sitemap
```

## Basic Usage

```go
package main

import (
	"log"
	"time"

	"github.com/jor-go/sitemap"
)

func main() {
	var sm sitemap.Sitemap

	for _, loc := range []string{
		"https://example.com/",
		"https://example.com/products",
	} {
		var u sitemap.URL
		if err := u.New(loc, "daily", 0.5, time.Now()); err != nil {
			log.Fatal(err)
		}
		sm.AddURL(u)
	}

	xmlBytes, err := sm.Generate()
	if err != nil {
		log.Fatal(err)
	}

	_ = xmlBytes
}
```

Use `URL` directly when you only need the required `<loc>` field:

```go
sm.AddURL(sitemap.URL{Loc: "https://example.com/about"})
```

## Write Output

Write one sitemap to any `io.Writer`:

```go
var buf bytes.Buffer

if err := sm.Write(&buf); err != nil {
	log.Fatal(err)
}
```

Save one sitemap to disk:

```go
if err := sm.GenerateAndSave("public/sitemap.xml"); err != nil {
	log.Fatal(err)
}
```

## Split Large Sitemaps

One sitemap can contain up to 50,000 URLs. When a sitemap has more than 50,000
URLs, use `GenerateFiles`, `GenerateAndSaveFiles`, or `WriteFiles` to create a
sitemap index plus child sitemap files.

```go
opts := sitemap.GenerateOptions{
	PublicBaseURL: "https://example.com/",
}

if err := sm.GenerateAndSaveFiles("public", opts); err != nil {
	log.Fatal(err)
}
```

For more than 50,000 URLs, this writes:

```text
public/sitemap.xml
public/sitemap-1.xml
public/sitemap-2.xml
```

In that case, `sitemap.xml` is the sitemap index and references the child files
with public URLs such as `https://example.com/sitemap-1.xml`.

Customize output names:

```go
opts := sitemap.GenerateOptions{
	PublicBaseURL:          "https://example.com/sitemaps/",
	IndexFilename:          "index.xml",
	SitemapFilenamePattern: "pages-%d.xml",
}
```

## Writer Factory

Use `WriteFiles` when generated XML should go somewhere other than the local
filesystem. The factory receives each generated filename and returns an
`io.WriteCloser`.

```go
opts := sitemap.GenerateOptions{
	PublicBaseURL: "https://example.com/",
}

err := sm.WriteFiles(opts, func(filename string) (io.WriteCloser, error) {
	return os.Create(filepath.Join("public", filename))
})
if err != nil {
	log.Fatal(err)
}
```

## Sitemap Indexes

You can also write an index directly:

```go
entries := []sitemap.SitemapRef{
	{Loc: "https://example.com/sitemap-1.xml", LastMod: "2026-05-13"},
	{Loc: "https://example.com/sitemap-2.xml"},
}

if err := sitemap.WriteIndex(os.Stdout, entries); err != nil {
	log.Fatal(err)
}
```

## Validation

The library validates:

- `loc` is required, absolute, includes a scheme and host, and is less than 2,048 characters.
- `changefreq`, when set, is one of `always`, `hourly`, `daily`, `weekly`, `monthly`, `yearly`, or `never`.
- `priority`, when set, is between `0.0` and `1.0`.
- `Generate` and `Write` return an error for more than 50,000 URLs; use split output APIs instead.

The Sitemaps protocol also limits each uncompressed sitemap or sitemap index to
50MB. This library does not enforce that byte-size limit.
