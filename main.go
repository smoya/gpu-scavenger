package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/kelseyhightower/envconfig"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/net/html"
	tb "gopkg.in/tucnak/telebot.v2"
)

type Specification struct {
	TelegramBotToken         string        `split_words:"true" required:"true"`
	TelegramNotificationChat string        `split_words:"true" required:"true"`
	Timeout                  time.Duration `default:"4s"`
	TickerMinTime            time.Duration `split_words:"true" default:"10s"`
	TickerMaxTime            time.Duration `split_words:"true" default:"20s"`
	RenotifyAfter            time.Duration `split_words:"true" default:"10m"`
	Debug                    bool          `default:"false"`
}

type site struct {
	Name              string
	URL               *url.URL
	responseReader    func(body io.Reader) []byte
	LinkXPathSelector string // XPath selector to the link (<a> html element or whatever) of any product
	Property          string // Property to grab. Default `href`
}

var sites = []site{
	{
		Name:              "ldlc.com",
		URL:               parseURL("https://www.ldlc.com/es-es/informatica/piezas-de-informatica/tarjeta-grafica/c4684/+fdi-1+fp-l49h958+fv1026-5801+fv121-19184,19365.html"),
		LinkXPathSelector: "//div[@class='pdt-desc']/h3/a",
	},
	{
		Name:              "Coolmod.com",
		URL:               parseURL("https://www.coolmod.com/tarjetas-gr%C3%A1ficas?f=9999::20077||571::RTX%203070||571::RTX%203080||571::RTX%203060%20Ti||prices::39-933||9995::relevance"),
		LinkXPathSelector: "//div[contains(text(), 'Envío')]/../div[1]/a",
	},
	{
		Name:              "VsGamers.es",
		URL:               parseURL("https://www.vsgamers.es/category/componentes/tarjetas-graficas?filter-modelo=rtxr-3060-ti-1268+rtxr-3070-1224+rtxr-3080-1225&to_price=921"),
		LinkXPathSelector: "//button[@class=\"btn btn-primary btn-block vs-product-card-buy\"]",
		Property:          "vs-cart-action",
	},
	{
		Name: "Neobyte.es",
		URL: parseURL("https://www.neobyte.es/modules/blocklayered_mod/blocklayered_mod-ajax.php?layered_id_feature_327=327_1084071049&layered_id_feature_289=289_1084071049&layered_id_feature_290=290_1084071049&id_category_layered=111&layered_price_slider=30_904&orderby=quantity&orderway=desc&n=32&_=1616156382767" +
			""),
		LinkXPathSelector: "//span[contains(text(), \"al carrito\")]/../../../../div[@class=\"right-block\"]/h5[@class=\"product-name-container\"]/a",
		responseReader: func(body io.Reader) []byte {
			// This code grabs HTML from the json as a response from an ajax call Neobyte.es does when filtering products
			r, _ := ioutil.ReadAll(body)
			if r[0] != '{' {
				logrus.WithField("response_first_char", string(r[0])).Warn("JSON response expected but it was not")
				return r
			}

			d := make(map[string]string)
			_ = json.Unmarshal(r, &d)

			if _, ok := d["productList"]; !ok {
				logrus.Warn("expected productList JSON field in response but was not present")
				return r
			}
			return []byte(d["productList"])
		},
	},
}

func main() {
	var conf Specification
	err := envconfig.Process("gpuscavenger", &conf)
	if err != nil {
		logrus.Fatal(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	handleInterruptions(cancel)

	if conf.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	b, err := tb.NewBot(tb.Settings{
		Token: conf.TelegramBotToken,
	})
	if err != nil {
		logrus.Fatal(err)
		return
	}

	tg, err := b.ChatByID(conf.TelegramNotificationChat)
	if err != nil {
		logrus.Fatal(err)
	}

	c := *http.DefaultClient
	c.Timeout = conf.Timeout

	// Create a cache with a default expiration time of (default) 10 minutes, and which purges expired items every (default) 20 minutes
	linksCache := cache.New(conf.RenotifyAfter, conf.RenotifyAfter*2)

	job(ctx, c, b, tg, linksCache) // First job does not wait

	for {
		rand.Seed(time.Now().UnixNano())
		randomExtraTime := time.Duration(rand.Intn(int(conf.TickerMaxTime.Seconds()-conf.TickerMinTime.Seconds()))) * time.Second
		secondsToWait := conf.TickerMinTime + randomExtraTime
		logrus.WithField("seconds", secondsToWait.Seconds()).Debug("Sleeping")

		time.Sleep(secondsToWait)
		job(ctx, c, b, tg, linksCache)
	}
}

func job(ctx context.Context, c http.Client, b *tb.Bot, tg *tb.Chat, linksCache *cache.Cache) {
	var wg sync.WaitGroup
	wg.Add(len(sites))
	for _, s := range sites {
		go func(s site) {
			defer wg.Done()
			scavenge(ctx, s, c, b, tg, linksCache)
		}(s)
	}

	wg.Wait()
}

func scavenge(ctx context.Context, s site, c http.Client, b *tb.Bot, tg *tb.Chat, linksCache *cache.Cache) {
	defer logrus.Debug("Finished scavenging at ", s.Name)
	logrus.Debug("Scavenging at ", s.Name)
	log := logrus.WithFields(logrus.Fields{
		"s":        s.Name,
		"url":      s.URL.String(),
		"selector": s.LinkXPathSelector,
	})

	resp, err := c.Do(generateRequest(ctx, s.URL.String()))
	if err != nil {
		log.Error("Error making http request")
		return
	}
	defer resp.Body.Close()

	var content *html.Node
	if s.responseReader != nil {
		content, err = htmlquery.Parse(bytes.NewBuffer(s.responseReader(resp.Body)))
	} else {
		content, err = htmlquery.Parse(resp.Body)
	}

	if err != nil {
		log.Warn("Invalid content")
		return
	}
	nodes, err := htmlquery.QueryAll(content, s.LinkXPathSelector)
	if err != nil {
		log.Warn("Error querying content through XPATH")
		return
	}

	if len(nodes) == 0 {
		log.WithField("reason", "selector_not_found").Info("No stock available for ", s.Name)
		return
	}

	productsAvailable := extractProducts(s, nodes, linksCache)
	if len(productsAvailable) == 0 {
		log.WithField("reason", "products_not_found").Info("No NEW stock available")
		return
	}

	toStr := fmt.Sprintf("Found new stock for:\n- %s", strings.Join(productsAvailable, "\n- "))
	logrus.Info(toStr)
	if _, err := b.Send(tg, toStr, tb.NoPreview); err != nil {
		logrus.Fatal(err)
	}
}

func extractProducts(s site, nodes []*html.Node, linksCache *cache.Cache) []string {
	property := s.Property
	if property == "" {
		property = "href"
	}

	var productsAvailable []string
	for _, n := range nodes {
		var link, title string
		for _, a := range n.Attr {
			switch a.Key {
			case property:
				absURL, err := url.Parse(a.Val)
				if err != nil || !absURL.IsAbs() {
					// If not absolute, generate it
					absURL, _ = url.Parse(fmt.Sprintf("%s://%s%s", s.URL.Scheme, s.URL.Host, a.Val))
				}

				link = absURL.String()
			case "title":
				title = a.Val
			}
		}

		if title == "" && n.FirstChild != nil {
			title = n.FirstChild.Data // Use text inside the <a> element
		}

		if _, found := linksCache.Get(link); found {
			logrus.Debug("Found new stock but notification is skipped as it was already notified")
			continue
		}

		linksCache.Set(link, struct{}{}, cache.DefaultExpiration)

		if title != "" {
			productsAvailable = append(productsAvailable, fmt.Sprintf("%s:\t%s", title, link))
		} else {
			productsAvailable = append(productsAvailable, link)
		}
	}

	return productsAvailable
}

func parseURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

func generateRequest(ctx context.Context, url string) *http.Request {
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	r.Header.Set("Connection", "keep-alive")
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4099.0 Safari/537.36")
	r.Header.Set("Referer", url)
	r.Header.Set("Accept-Language", "es,es-ES;q=0.9,es;q=0.8,fr;q=0.7")

	return r
}

func handleInterruptions(cancel context.CancelFunc) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-c
		logrus.WithField("signal", s).Info("Stopping the scavenger due to received signal...")
		cancel()
		time.Sleep(time.Second)
		os.Exit(0)
	}()
}
