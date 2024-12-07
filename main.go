package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

type Headers struct {
	values []string
}

type Selectors struct {
	body    string
	title   string
	snippet string
	link    string
	authors string
	next    string
}

type ScraperService struct {
	baseUrl      string
	collectorUrl string
	selectors    *Selectors
	maxPages     int
	outputDir    string
	headers      *Headers
}

func NewScraperService() *ScraperService {
	return &ScraperService{
		baseUrl:      "https://scholar.google.com/scholar",
		collectorUrl: "scholar.google.com",
		selectors: &Selectors{
			body:    ".gs_r",
			title:   ".gs_rt",
			snippet: ".gs_rs",
			link:    ".gs_rt a",
			authors: ".gs_a",
			next:    "#gs_n td a",
		},
		maxPages:  100,
		outputDir: "output",
		headers: &Headers{
			values: []string{
				"Title",
				"Snippet",
				"Link",
				"Authors",
				"Date",
				"DOI",
				"Journal",
				"Cited by",
				"All versions",
				"Page",
			},
		},
	}
}

func (s *ScraperService) Scrape(c *colly.Collector, writer *csv.Writer, url string, term string, currentPage *int, maxPages int) {
	var citations, totalCitations int
	var lastProcessedPage int = -1

	c.OnHTML(s.selectors.body, func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.ChildText(".gs_rt"))
		snippet := strings.TrimSpace(e.ChildText(".gs_rs"))
		link := strings.TrimSpace(e.ChildAttr(".gs_rt a", "href"))
		authors := strings.TrimSpace(e.ChildText(".gs_a"))
		date := extractDate(authors)
		doi := extractDOI(link)
		journal := extractJournal(authors)
		citedBy := extractCitedBy(e)
		allVersions := extractAllVersions(e)

		if title == "" && snippet == "" {
			return
		}
		if strings.Contains(strings.ToLower(title), strings.ToLower(term)) ||
			strings.Contains(strings.ToLower(snippet), strings.ToLower(term)) {
			err := writer.Write([]string{title, snippet, link, authors, date, doi, journal, citedBy, allVersions, fmt.Sprintf("%d", *currentPage+1)})
			if err != nil {
				log.Printf("Failed to write CSV record: %v", err)
			} else {
				citations++
				totalCitations++
			}
			writer.Flush()
		}
	})

	c.OnHTML(s.selectors.next, func(e *colly.HTMLElement) {
		if strings.Contains(e.Text, "Next") && *currentPage < maxPages {
			if lastProcessedPage != *currentPage {
				log.Printf("Page %d scraped.", *currentPage+1)
				lastProcessedPage = *currentPage
			}

			nextPage := e.Attr("href")
			*currentPage++
			log.Printf("Navigating to page %d...", *currentPage+1)

			citations = 0
			err := e.Request.Visit(nextPage)
			if err != nil {
				log.Printf("Error visiting next page: %v", err)
			}
		}
	})

	c.OnScraped(func(r *colly.Response) {
		if lastProcessedPage != *currentPage {
			lastProcessedPage = *currentPage
			log.Printf("Found %d citations on page %d", totalCitations, *currentPage)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Request failed on URL: %s, Error: %v", r.Request.URL, err)
	})

	err := c.Visit(url)
	if err != nil {
		log.Fatalf("Failed to start scraping: %v", err)
	}

	c.Wait()
	log.Printf("Total results found: %d", totalCitations)
}

func (s *ScraperService) flags() (string, string, string, bool) {
	term := flag.String("query", "", "Search term for Google Scholar")
	lang := flag.String("lang", "en", "Language (default en)")
	sdt := flag.String("sdt", "0,5", "Scholar document type (0,5=All, 0,33=Articles, 1,5=Case law, 0=No patents, 2=Patents only)")
	slow := flag.Bool("slow", false, "Enable 'slow mode', lower request rate for extra caution")
	flag.Parse()
	if *term == "" {
		log.Fatal("Error: Please provide a search term using -query flag followed by a search term (word)")
	}
	return *term, *lang, *sdt, *slow
}

func (s *ScraperService) createCollector(slow bool) *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains(s.collectorUrl),
		colly.Async(true),
		colly.MaxDepth(100),
	)
	delay := getBaseDelay(slow)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*scholar.google.com*",
		Parallelism: 1,
		Delay:       delay,
	})
	return c
}

func (s *ScraperService) writeHeaders(writer *csv.Writer, headers *Headers) {
	err := writer.Write(headers.values)
	if err != nil {
		log.Fatalf("Failed to write headers to file: %v", err)
	}
}

func (s *ScraperService) constructURL(term string, page int, lang string, sdt string) string {
	return fmt.Sprintf("%s?start=%d&q=%s&hl=%s&as_sdt=%s", s.baseUrl, page*10, term, lang, sdt)
}

func (s *ScraperService) createOutputFile(term string) (*os.File, string) {
	path, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get exe path: %v", err)
	}

	dir := filepath.Dir(path)
	outputDir := filepath.Join(dir, s.outputDir)

	if err := os.MkdirAll(outputDir, 0700); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	filePath := s.buildAbsolutePath(term)
	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("Failed to create the output file: %v", err)
	}

	return file, filePath
}

func (s *ScraperService) buildAbsolutePath(term string) string {
	stamp := time.Now().Format("20060102-150405")
	fileName := fmt.Sprintf("scrape-%s-%s.csv", term, stamp)

	return filepath.Join(s.outputDir, fileName)
}

func extractDate(authors string) string {
	re := regexp.MustCompile(`(19|20)\d{2}`)
	if match := re.FindString(authors); match != "" {
		return match
	}

	parts := strings.Split(authors, "-")
	if len(parts) > 1 {
		lastPart := strings.TrimSpace(parts[len(parts)-1])
		if match := re.FindString(lastPart); match != "" {
			return match
		}
		return lastPart
	}

	return "Unknown"
}

func extractDOI(link string) string {
	if strings.Contains(link, "doi.org") {
		return link
	}
	return "N/A"
}

func extractJournal(authors string) string {
	parts := strings.Split(authors, "-")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[0])
	}
	return "Unknown"
}

func extractCitedBy(e *colly.HTMLElement) string {
	citedBy := e.ChildText(".gs_fl a")
	if strings.Contains(citedBy, "Cited by") {
		re := regexp.MustCompile(`Cited by (\d+)`)
		matches := re.FindStringSubmatch(citedBy)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return "0"
}

func getBaseDelay(slow bool) time.Duration {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	baseDelay := time.Duration(r.Intn(5)+1) * time.Second
	if slow {
		baseDelay += time.Duration(r.Intn(10)+5) * time.Second
	}
	return baseDelay
}

func extractAllVersions(e *colly.HTMLElement) string {
	allVersions := e.ChildText(".gs_fl a")
	if strings.Contains(allVersions, "All") {
		re := regexp.MustCompile(`All (\d+) versions`)
		matches := re.FindStringSubmatch(allVersions)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return "0"
}

func main() {
	service := NewScraperService()
	term, lang, sdt, slowMode := service.flags()

	page := 0
	url := service.constructURL(term, page, lang, sdt)
	log.Printf("Scraping URL: %s", url)

	file, fileName := service.createOutputFile(term)
	writer := csv.NewWriter(file)
	defer file.Close()

	service.writeHeaders(writer, service.headers)
	collector := service.createCollector(slowMode)

	service.Scrape(collector, writer, url, term, &page, service.maxPages)

	log.Printf("Scrape complete. It navigated through %d pages. The results were saved to a CSV file: %s\n", page, fileName)
}
