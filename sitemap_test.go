package sitemap

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGenerateSitemapXML(t *testing.T) {
	sm := Sitemap{}
	sm.AddURL(URL{
		Loc:        "https://example.com/catalog?item=12&desc=vacation",
		LastMod:    "2026-05-13",
		ChangeFreq: "daily",
		Priority:   "0.8",
	})

	data, err := sm.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	got := string(data)
	if !strings.HasPrefix(got, xml.Header) {
		t.Fatalf("Generate() missing XML header: %q", got)
	}
	if !strings.Contains(got, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`) {
		t.Fatalf("Generate() missing urlset namespace: %s", got)
	}
	if !strings.Contains(got, "<loc>https://example.com/catalog?item=12&amp;desc=vacation</loc>") {
		t.Fatalf("Generate() did not XML-escape loc: %s", got)
	}
	if !strings.Contains(got, "<lastmod>2026-05-13</lastmod>") {
		t.Fatalf("Generate() missing lastmod: %s", got)
	}
	if !strings.Contains(got, "<changefreq>daily</changefreq>") {
		t.Fatalf("Generate() missing changefreq: %s", got)
	}
	if !strings.Contains(got, "<priority>0.8</priority>") {
		t.Fatalf("Generate() missing priority: %s", got)
	}
}

func TestGenerateOmitsOptionalFields(t *testing.T) {
	sm := Sitemap{}
	sm.AddURL(URL{Loc: "https://example.com/"})

	data, err := sm.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	got := string(data)
	for _, tag := range []string{"lastmod", "changefreq", "priority"} {
		if strings.Contains(got, "<"+tag+">") {
			t.Fatalf("Generate() included optional tag %q: %s", tag, got)
		}
	}
}

func TestWriteMatchesGenerate(t *testing.T) {
	sm := Sitemap{}
	sm.AddURL(URL{Loc: "https://example.com/"})

	want, err := sm.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var got bytes.Buffer
	if err := sm.Write(&got); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("Write() != Generate()\nwant:\n%s\ngot:\n%s", want, got.Bytes())
	}
}

func TestGenerateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		sm   Sitemap
		want string
	}{
		{
			name: "empty sitemap",
			sm:   Sitemap{},
			want: "no URLs",
		},
		{
			name: "relative URL",
			sm:   Sitemap{URLS: []URL{{Loc: "/relative"}}},
			want: "absolute URL",
		},
		{
			name: "invalid changefreq",
			sm:   Sitemap{URLS: []URL{{Loc: "https://example.com/", ChangeFreq: "sometimes"}}},
			want: "invalid changefreq",
		},
		{
			name: "invalid priority",
			sm:   Sitemap{URLS: []URL{{Loc: "https://example.com/", Priority: "1.2"}}},
			want: "invalid priority",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.sm.Generate()
			if err == nil {
				t.Fatalf("Generate() error = nil, want containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Generate() error = %q, want containing %q", err, tt.want)
			}
		})
	}
}

func TestGenerateRejectsMoreThanMaxURLs(t *testing.T) {
	sm := sitemapWithURLs(MaxURLsPerSitemap + 1)

	_, err := sm.Generate()
	if err == nil {
		t.Fatal("Generate() error = nil, want max URL error")
	}
	if !strings.Contains(err.Error(), "maximum is 50000") {
		t.Fatalf("Generate() error = %q, want max URL error", err)
	}
}

func TestGenerateFilesAtMaxURLsProducesSingleFile(t *testing.T) {
	sm := sitemapWithURLs(MaxURLsPerSitemap)

	files, err := sm.GenerateFiles(GenerateOptions{})
	if err != nil {
		t.Fatalf("GenerateFiles() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("GenerateFiles() file count = %d, want 1", len(files))
	}
	if files[0].Name != "sitemap.xml" {
		t.Fatalf("GenerateFiles() name = %q, want sitemap.xml", files[0].Name)
	}
	if strings.Contains(string(files[0].Data), "<sitemapindex") {
		t.Fatalf("GenerateFiles() produced index at max URL boundary")
	}
}

func TestGenerateFilesSplitsAndCreatesIndex(t *testing.T) {
	sm := sitemapWithURLs(MaxURLsPerSitemap + 1)

	files, err := sm.GenerateFiles(GenerateOptions{
		PublicBaseURL:          "https://example.com/sitemaps",
		SitemapFilenamePattern: "site-%d.xml",
	})
	if err != nil {
		t.Fatalf("GenerateFiles() error = %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("GenerateFiles() file count = %d, want 3", len(files))
	}

	names := []string{files[0].Name, files[1].Name, files[2].Name}
	wantNames := []string{"sitemap.xml", "site-1.xml", "site-2.xml"}
	for i := range wantNames {
		if names[i] != wantNames[i] {
			t.Fatalf("GenerateFiles() names = %v, want %v", names, wantNames)
		}
	}

	index := string(files[0].Data)
	if !strings.Contains(index, "<sitemapindex") {
		t.Fatalf("GenerateFiles() index missing sitemapindex: %s", index)
	}
	if !strings.Contains(index, "<loc>https://example.com/sitemaps/site-1.xml</loc>") {
		t.Fatalf("GenerateFiles() index missing first child loc: %s", index)
	}
	if !strings.Contains(index, "<loc>https://example.com/sitemaps/site-2.xml</loc>") {
		t.Fatalf("GenerateFiles() index missing second child loc: %s", index)
	}
	if strings.Count(string(files[1].Data), "<url>") != MaxURLsPerSitemap {
		t.Fatalf("first child URL count = %d, want %d", strings.Count(string(files[1].Data), "<url>"), MaxURLsPerSitemap)
	}
	if strings.Count(string(files[2].Data), "<url>") != 1 {
		t.Fatalf("second child URL count = %d, want 1", strings.Count(string(files[2].Data), "<url>"))
	}
}

func TestGenerateAndSave(t *testing.T) {
	sm := Sitemap{URLS: []URL{{Loc: "https://example.com/"}}}
	path := filepath.Join(t.TempDir(), "sitemap.xml")

	if err := sm.GenerateAndSave(path); err != nil {
		t.Fatalf("GenerateAndSave() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "<urlset") {
		t.Fatalf("GenerateAndSave() wrote unexpected content: %s", data)
	}
}

func TestGenerateAndSaveFiles(t *testing.T) {
	sm := sitemapWithURLs(MaxURLsPerSitemap + 1)
	dir := t.TempDir()

	err := sm.GenerateAndSaveFiles(dir, GenerateOptions{
		PublicBaseURL: "https://example.com/",
	})
	if err != nil {
		t.Fatalf("GenerateAndSaveFiles() error = %v", err)
	}

	for _, name := range []string{"sitemap.xml", "sitemap-1.xml", "sitemap-2.xml"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s: %v", name, err)
		}
	}
}

func TestWriteFilesUsesWriterFactory(t *testing.T) {
	sm := sitemapWithURLs(MaxURLsPerSitemap + 1)
	writers := map[string]*memoryWriteCloser{}

	err := sm.WriteFiles(GenerateOptions{
		PublicBaseURL: "https://example.com/",
	}, func(filename string) (io.WriteCloser, error) {
		w := &memoryWriteCloser{}
		writers[filename] = w
		return w, nil
	})
	if err != nil {
		t.Fatalf("WriteFiles() error = %v", err)
	}

	for _, name := range []string{"sitemap.xml", "sitemap-1.xml", "sitemap-2.xml"} {
		writer, ok := writers[name]
		if !ok {
			t.Fatalf("WriteFiles() did not create writer for %s", name)
		}
		if writer.closed == false {
			t.Fatalf("WriteFiles() did not close writer for %s", name)
		}
		if writer.Len() == 0 {
			t.Fatalf("WriteFiles() wrote no data for %s", name)
		}
	}
}

func TestWriteFilesClosesAfterWriteError(t *testing.T) {
	sm := Sitemap{URLS: []URL{{Loc: "https://example.com/"}}}
	writer := &failingWriteCloser{}

	err := sm.WriteFiles(GenerateOptions{}, func(filename string) (io.WriteCloser, error) {
		return writer, nil
	})
	if err == nil {
		t.Fatal("WriteFiles() error = nil, want write error")
	}
	if !writer.closed {
		t.Fatal("WriteFiles() did not close writer after write error")
	}
}

func TestURLNew(t *testing.T) {
	var u URL
	err := u.New("https://example.com/", "monthly", 0.5, time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("URL.New() error = %v", err)
	}
	if u.LastMod != "2026-05-13" {
		t.Fatalf("URL.New() LastMod = %q, want 2026-05-13", u.LastMod)
	}
	if u.Priority != "0.5" {
		t.Fatalf("URL.New() Priority = %q, want 0.5", u.Priority)
	}
}

func TestWriteIndex(t *testing.T) {
	var buf bytes.Buffer
	err := WriteIndex(&buf, []SitemapRef{
		{Loc: "https://example.com/sitemap-1.xml", LastMod: "2026-05-13"},
	})
	if err != nil {
		t.Fatalf("WriteIndex() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "<sitemapindex") {
		t.Fatalf("WriteIndex() missing sitemapindex: %s", got)
	}
	if !strings.Contains(got, "<lastmod>2026-05-13</lastmod>") {
		t.Fatalf("WriteIndex() missing lastmod: %s", got)
	}
}

func sitemapWithURLs(count int) Sitemap {
	sm := Sitemap{URLS: make([]URL, 0, count)}
	for i := 0; i < count; i++ {
		sm.URLS = append(sm.URLS, URL{Loc: "https://example.com/page/" + strconv.Itoa(i)})
	}
	return sm
}

type memoryWriteCloser struct {
	bytes.Buffer
	closed bool
}

func (m *memoryWriteCloser) Close() error {
	m.closed = true
	return nil
}

type failingWriteCloser struct {
	closed bool
}

func (f *failingWriteCloser) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func (f *failingWriteCloser) Close() error {
	f.closed = true
	return nil
}
