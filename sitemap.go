package sitemap

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// Namespace is the XML namespace for the Sitemaps protocol.
	Namespace = "http://www.sitemaps.org/schemas/sitemap/0.9"

	// MaxURLsPerSitemap is the maximum number of URLs allowed in one sitemap.
	MaxURLsPerSitemap = 50000

	// MaxSitemapsPerIndex is the maximum number of sitemap entries allowed in one sitemap index.
	MaxSitemapsPerIndex = 50000

	maxLocLength = 2048
)

var validFrequencies = map[string]struct{}{
	"always":  {},
	"hourly":  {},
	"daily":   {},
	"weekly":  {},
	"monthly": {},
	"yearly":  {},
	"never":   {},
}

var (
	errNoURLs       = errors.New("sitemap: no URLs in sitemap")
	errNilWriter    = errors.New("sitemap: writer is nil")
	errNoIndexRefs  = errors.New("sitemap: no sitemap references in index")
	errNilFactory   = errors.New("sitemap: writer factory is nil")
	errNoPublicBase = errors.New("sitemap: public base URL is required when generating a sitemap index")
)

// Sitemap holds all the URLs in the sitemap.
type Sitemap struct {
	Format  string   `xml:"-"`
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr,omitempty"`
	URLS    []URL    `xml:"url"`
}

// URL is a URL entry for the sitemap.
type URL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

// SitemapRef is an entry in a sitemap index file.
type SitemapRef struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// GenerateOptions controls generated sitemap file names and index locations.
type GenerateOptions struct {
	// PublicBaseURL is the public URL directory used to build child sitemap
	// locations in the sitemap index, for example "https://example.com/".
	PublicBaseURL string

	// IndexFilename is the generated index filename. It defaults to "sitemap.xml".
	IndexFilename string

	// SitemapFilenamePattern is used with fmt.Sprintf and a 1-based index for
	// child sitemap filenames. It defaults to "sitemap-%d.xml".
	SitemapFilenamePattern string
}

// GeneratedFile is an XML sitemap or sitemap index generated in memory.
type GeneratedFile struct {
	Name string
	Data []byte
}

// WriterFactory creates a writer for a generated sitemap filename.
type WriterFactory func(filename string) (io.WriteCloser, error)

type urlSet struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URLS    []URL    `xml:"url"`
}

type sitemapIndex struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	XMLNS    string       `xml:"xmlns,attr"`
	Sitemaps []SitemapRef `xml:"sitemap"`
}

// AddURL adds a single URL to the sitemap.
func (s *Sitemap) AddURL(u URL) {
	s.URLS = append(s.URLS, u)
}

// AddURLs adds multiple URLs to the sitemap.
func (s *Sitemap) AddURLs(u []URL) {
	s.URLS = append(s.URLS, u...)
}

// Generate creates a single sitemap XML document.
func (s *Sitemap) Generate() ([]byte, error) {
	return s.generateURLSet(s.URLS, true)
}

// Write writes a single sitemap XML document to w.
func (s *Sitemap) Write(w io.Writer) error {
	if w == nil {
		return errNilWriter
	}

	data, err := s.Generate()
	if err != nil {
		return err
	}

	_, err = io.Copy(w, bytes.NewReader(data))
	return err
}

// GenerateAndSave generates a single sitemap XML document and saves it to path.
func (s *Sitemap) GenerateAndSave(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	writeErr := s.Write(file)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// GenerateFiles creates sitemap files in memory. When the sitemap has more than
// MaxURLsPerSitemap URLs, sitemap.xml is a sitemap index and child sitemaps are
// generated using SitemapFilenamePattern.
func (s *Sitemap) GenerateFiles(opts GenerateOptions) ([]GeneratedFile, error) {
	if len(s.URLS) <= MaxURLsPerSitemap {
		data, err := s.Generate()
		if err != nil {
			return nil, err
		}
		return []GeneratedFile{{Name: opts.indexFilename(), Data: data}}, nil
	}

	if err := validateURLs(s.URLS, false); err != nil {
		return nil, err
	}
	if err := opts.validatePublicBaseURL(); err != nil {
		return nil, err
	}

	chunkCount := (len(s.URLS) + MaxURLsPerSitemap - 1) / MaxURLsPerSitemap
	if chunkCount > MaxSitemapsPerIndex {
		return nil, fmt.Errorf("sitemap: sitemap index would contain %d sitemaps, maximum is %d", chunkCount, MaxSitemapsPerIndex)
	}

	files := make([]GeneratedFile, 0, chunkCount+1)
	refs := make([]SitemapRef, 0, chunkCount)

	for i, start := 0, 0; start < len(s.URLS); i, start = i+1, start+MaxURLsPerSitemap {
		end := start + MaxURLsPerSitemap
		if end > len(s.URLS) {
			end = len(s.URLS)
		}

		name, err := opts.sitemapFilename(i + 1)
		if err != nil {
			return nil, err
		}

		loc, err := opts.publicURLFor(name)
		if err != nil {
			return nil, err
		}

		data, err := s.generateURLSet(s.URLS[start:end], true)
		if err != nil {
			return nil, err
		}

		files = append(files, GeneratedFile{Name: name, Data: data})
		refs = append(refs, SitemapRef{Loc: loc})
	}

	indexData, err := marshalIndex(refs, s.format(), s.namespace())
	if err != nil {
		return nil, err
	}

	return append([]GeneratedFile{{Name: opts.indexFilename(), Data: indexData}}, files...), nil
}

// GenerateAndSaveFiles writes generated sitemap files to dir.
func (s *Sitemap) GenerateAndSaveFiles(dir string, opts GenerateOptions) error {
	return s.WriteFiles(opts, func(filename string) (io.WriteCloser, error) {
		path := filepath.Join(dir, filename)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		return os.Create(path)
	})
}

// WriteFiles writes generated sitemap files using create.
func (s *Sitemap) WriteFiles(opts GenerateOptions, create WriterFactory) error {
	if create == nil {
		return errNilFactory
	}

	files, err := s.GenerateFiles(opts)
	if err != nil {
		return err
	}

	for _, file := range files {
		writer, err := create(file.Name)
		if err != nil {
			return fmt.Errorf("sitemap: create writer for %q: %w", file.Name, err)
		}

		_, writeErr := io.Copy(writer, bytes.NewReader(file.Data))
		closeErr := writer.Close()
		if writeErr != nil {
			return fmt.Errorf("sitemap: write %q: %w", file.Name, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("sitemap: close %q: %w", file.Name, closeErr)
		}
	}

	return nil
}

// TimeToLastMod converts a time.Time to the last modified date as a string.
func (u *URL) TimeToLastMod(t time.Time) {
	u.LastMod = t.Format("2006-01-02")
}

// New creates a new URL for the sitemap.
//
// location: URL e.g. https://...
//
// changeFrequency: "always", "hourly", "daily", "weekly", "monthly", "yearly", or "never"
//
// priority: float between 0.0 and 1.0
//
// lastModified: time.Time of the last time link was modified
func (u *URL) New(location, changeFrequency string, priority float64, lastModified time.Time) error {
	if err := validateLoc(location); err != nil {
		return fmt.Errorf("sitemap.URL.New() : Invalid URL -> %s", location)
	}

	if _, ok := validFrequencies[changeFrequency]; !ok {
		return errors.New("sitemap.URL.New() : Invalid changeFrequency")
	}

	if priority > 1.0 || priority < 0.0 {
		return errors.New("sitemap.URL.New() : Invalid priority - Must be between 0.0 and 1.0")
	}

	u.Loc = location
	u.TimeToLastMod(lastModified)
	u.ChangeFreq = changeFrequency
	u.Priority = strconv.FormatFloat(priority, 'f', 1, 64)

	return nil
}

// GenerateIndex creates a sitemap index XML document.
func GenerateIndex(entries []SitemapRef) ([]byte, error) {
	return marshalIndex(entries, xml.Header, Namespace)
}

// WriteIndex writes a sitemap index XML document to w.
func WriteIndex(w io.Writer, entries []SitemapRef) error {
	if w == nil {
		return errNilWriter
	}

	data, err := GenerateIndex(entries)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, bytes.NewReader(data))
	return err
}

func (s *Sitemap) generateURLSet(urls []URL, enforceLimit bool) ([]byte, error) {
	if err := validateURLs(urls, enforceLimit); err != nil {
		return nil, err
	}

	return marshalXML(s.format(), urlSet{
		XMLNS: s.namespace(),
		URLS:  urls,
	})
}

func (s *Sitemap) format() string {
	if s.Format != "" {
		return s.Format
	}
	return xml.Header
}

func (s *Sitemap) namespace() string {
	if s.XMLNS != "" {
		return s.XMLNS
	}
	return Namespace
}

func marshalIndex(entries []SitemapRef, header, namespace string) ([]byte, error) {
	if len(entries) == 0 {
		return nil, errNoIndexRefs
	}
	if len(entries) > MaxSitemapsPerIndex {
		return nil, fmt.Errorf("sitemap: sitemap index contains %d sitemaps, maximum is %d", len(entries), MaxSitemapsPerIndex)
	}

	for i, entry := range entries {
		if err := validateLoc(entry.Loc); err != nil {
			return nil, fmt.Errorf("sitemap: invalid sitemap index entry %d: %w", i, err)
		}
	}

	return marshalXML(header, sitemapIndex{
		XMLNS:    namespace,
		Sitemaps: entries,
	})
}

func marshalXML(header string, value any) ([]byte, error) {
	body, err := xml.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("sitemap: marshal XML: %w", err)
	}

	var out bytes.Buffer
	out.WriteString(header)
	out.Write(body)
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func validateURLs(urls []URL, enforceLimit bool) error {
	if len(urls) == 0 {
		return errNoURLs
	}
	if enforceLimit && len(urls) > MaxURLsPerSitemap {
		return fmt.Errorf("sitemap: sitemap contains %d URLs, maximum is %d", len(urls), MaxURLsPerSitemap)
	}

	for i, u := range urls {
		if err := validateURL(u); err != nil {
			return fmt.Errorf("sitemap: invalid URL entry %d: %w", i, err)
		}
	}

	return nil
}

func validateURL(u URL) error {
	if err := validateLoc(u.Loc); err != nil {
		return err
	}

	if u.ChangeFreq != "" {
		if _, ok := validFrequencies[u.ChangeFreq]; !ok {
			return fmt.Errorf("invalid changefreq %q", u.ChangeFreq)
		}
	}

	if u.Priority != "" {
		priority, err := strconv.ParseFloat(u.Priority, 64)
		if err != nil {
			return fmt.Errorf("invalid priority %q", u.Priority)
		}
		if priority < 0.0 || priority > 1.0 {
			return fmt.Errorf("invalid priority %q", u.Priority)
		}
	}

	return nil
}

func validateLoc(loc string) error {
	if loc == "" {
		return errors.New("loc is required")
	}
	if len(loc) >= maxLocLength {
		return fmt.Errorf("loc must be less than %d characters", maxLocLength)
	}

	parsed, err := url.ParseRequestURI(loc)
	if err != nil {
		return fmt.Errorf("invalid loc %q", loc)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("loc must be an absolute URL with scheme and host: %q", loc)
	}

	return nil
}

func (opts GenerateOptions) indexFilename() string {
	if opts.IndexFilename != "" {
		return opts.IndexFilename
	}
	return "sitemap.xml"
}

func (opts GenerateOptions) sitemapFilename(index int) (string, error) {
	pattern := opts.SitemapFilenamePattern
	if pattern == "" {
		pattern = "sitemap-%d.xml"
	}
	if !strings.Contains(pattern, "%") {
		return "", errors.New("sitemap: sitemap filename pattern must contain an integer formatting verb")
	}

	name := fmt.Sprintf(pattern, index)
	if name == "" || strings.Contains(name, "%!") {
		return "", fmt.Errorf("sitemap: invalid sitemap filename pattern %q", pattern)
	}

	return name, nil
}

func (opts GenerateOptions) validatePublicBaseURL() error {
	if opts.PublicBaseURL == "" {
		return errNoPublicBase
	}

	parsed, err := url.ParseRequestURI(opts.PublicBaseURL)
	if err != nil {
		return fmt.Errorf("sitemap: invalid public base URL %q", opts.PublicBaseURL)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("sitemap: public base URL must be absolute with scheme and host: %q", opts.PublicBaseURL)
	}

	return nil
}

func (opts GenerateOptions) publicURLFor(filename string) (string, error) {
	parsed, err := url.Parse(opts.PublicBaseURL)
	if err != nil {
		return "", fmt.Errorf("sitemap: invalid public base URL %q", opts.PublicBaseURL)
	}

	if parsed.Path == "" {
		parsed.Path = "/"
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	parsed.Path += filename

	loc := parsed.String()
	if err := validateLoc(loc); err != nil {
		return "", err
	}

	return loc, nil
}
