package sitemap

import (
	"encoding/xml"
	"errors"
	"io/ioutil"
	"net/url"
	"strconv"
	"time"
)

var validFrequencies = []string{"always", "hourly", "daily", "weekly", "monthly", "yearly", "never"}
var defaultXMLNS = "http://www.sitemaps.org/schemas/sitemap/0.9"

/*Sitemap : Holds all the urls in the sitemap*/
type Sitemap struct {
	Format  string   `xml:",innerxml"`
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URLS    []URL    `xml:"url"`
}

/*AddURL : Adds a single URL to the sitemap*/
func (s *Sitemap) AddURL(u URL) {
	s.URLS = append(s.URLS, u)
}

/*AddURLs : Adds multiple URLs to the Sitemap*/
func (s *Sitemap) AddURLs(u []URL) {
	s.URLS = append(s.URLS, u...)
}

/*Generate : Creates sitemap []byte*/
func (s *Sitemap) Generate() ([]byte, error) {
	if s.Format == "" {
		s.Format = ""
	}

	if s.XMLNS == "" {
		s.XMLNS = defaultXMLNS
	}

	if len(s.URLS) <= 0 {
		return []byte{}, errors.New("sitemap.Sitemap.Generate() : No URLs in Sitemap")
	}

	data, err := xml.Marshal(s)

	if err != nil {
		return []byte{}, errors.New("sitemap.Sitemap.Generate() : Problem Marshaling XML -> " + err.Error())
	}

	header := []byte(xml.Header)

	final := append(header, data...)

	return final, nil
}

/*GenerateAndSave : Generates and saves to filepath e.g. "/path/to/your/sitemap.xml"*/
func (s *Sitemap) GenerateAndSave(path string) error {
	bytes, err := s.Generate()

	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, bytes, 666)
	if err != nil {
		return err
	}

	return nil
}

/*URL : is a url for the sitemap*/
type URL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

/*TimeToLastMod : Convert a time.Time to the last modified date as a string*/
func (u *URL) TimeToLastMod(t time.Time) {
	u.LastMod = t.Format("2006-01-02")
}

/*New : Creates a new URL for the sitemap.

location: URL e.g. https://...

changeFrequency: "always", "hourly", "daily", "weekly", "monthly", "yearly", or "never"

priority: float between 0.0 and 1.0

lastModified: time.Time of the last time link was modified
*/
func (u *URL) New(location, changeFrequency string, priority float64, lastModified time.Time) error {
	// make sure location is a valid URL
	if _, err := url.ParseRequestURI(location); err != nil {
		return errors.New("sitemap.URL.New() : Invalid URL -> " + location)
	}

	// make sure changeFrequency is a valid frequency type
	validFreq := false
	for _, freq := range validFrequencies {
		if changeFrequency == freq {
			validFreq = true
			break
		}
	}

	if !validFreq {
		return errors.New("sitemap.URL.New() : Invalid changeFrequency")
	}

	// valid priority
	if priority > 1.0 || priority < 0.0 {
		return errors.New("sitemap.URL.New() : Invalid priority - Must be between 0.0 and 1.0")
	}

	u.Loc = location
	u.TimeToLastMod(lastModified)
	u.ChangeFreq = changeFrequency
	u.Priority = strconv.FormatFloat(priority, 'f', 1, 32)

	return nil
}
