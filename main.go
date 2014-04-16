package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/darkhelmet/twitterstream"

	"github.com/robfig/config"
)

var (
	// twitter API keys
	consumerKey    string
	consumerSecret string
	accessToken    string
	accessSecret   string
	// or read from file
	twitterConfigFile string

	// filename for saving raw tweets
	outputFileName string

	// read keywords from file
	keywordFileName string
	// or directly from commandline
	keywordsCLI string
	// parsed keywords
	keywords = make([]string, 0)

	// port for webserver
	port string

	// log tweets to stdout
	verbose bool
)

func init() {
	flag.StringVar(&consumerKey, "consumer_key", "", "Twitter Consumer Key")
	flag.StringVar(&consumerSecret, "consumer_secret", "", "Twitter Consumer Secret")
	flag.StringVar(&accessToken, "access_token", "", "Twitter Access Token")
	flag.StringVar(&accessSecret, "access_secret", "", "Twitter Access Token Secret")
	flag.StringVar(&twitterConfigFile, "twitter_keys_file", "twitter_keys.cfg", "location of twitter keys file")
	flag.StringVar(&outputFileName, "output", "tweets.json", "location to save tweets")
	flag.StringVar(&keywordFileName, "keyword_file", "", "location of file specifying keywords")
	flag.StringVar(&keywordsCLI, "keywords", "", "comma separated list of keywords to track")
	flag.StringVar(&port, "port", "3000", "port to listen on")
	flag.BoolVar(&verbose, "verbose", false, "log tweets to stdout")
	flag.Parse()

	// check args and settings
	if consumerKey == "" && consumerSecret == "" && accessToken == "" && accessSecret == "" {
		err := readTwitterConfig(twitterConfigFile)
		if err != nil {
			log.Fatalf("error reading config from %v: %v", twitterConfigFile, err)
		}
	}

	if keywordFileName != "" {
		err := readKeywordFile(keywordFileName)
		if err != nil {
			log.Fatalf("error reading keywords from %v: %v", keywordFileName, err)
		}
	} else {
		readKeywordCLI(keywordsCLI)
	}

	checkConfig()
}

func main() {
	quit := make(chan bool)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, os.Kill)
	go watchSig(quit, sigchan)

	subscribers := make([]chan *twitterstream.Tweet, 0)

	// start file save goroutine
	saveChan, saveDone, err := tweetSaver(outputFileName)
	if err != nil {
		log.Fatalf("error opening %v for writing", err)
	}
	subscribers = append(subscribers, saveChan)

	// start web server
	wsChan := startServer(port)
	subscribers = append(subscribers, wsChan)
	// write tweets to command line
	if verbose {
		logChan := tweetLogger()
		subscribers = append(subscribers, logChan)
	}
	// connect to twitter api
	tweets := connectStream(keywords, quit)
	// connect save and server to tweet chan
	broadcastTweets(tweets, subscribers...) // this will block until tweets is closed
	// wait for save to complete
	<-saveDone
	// exit
	log.Println("finished")
}

// wait for os sig, close quit chan
func watchSig(quit chan bool, sigchan chan os.Signal) {
	<-sigchan
	log.Println("closing up...")
	close(quit)
}

func tweetSaver(filename string) (chan *twitterstream.Tweet, chan bool, error) {
	tweets := make(chan *twitterstream.Tweet, 100) // buffered
	done := make(chan bool)

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return tweets, done, err
	}

	go func() {
		w := json.NewEncoder(f)
		for tweet := range tweets {
			w.Encode(tweet)
		}
		f.Close()
		close(done) // signal finished writing
	}()
	return tweets, done, nil
}

func tweetLogger() chan *twitterstream.Tweet {
	tweets := make(chan *twitterstream.Tweet, 100)

	go func() {
		for tweet := range tweets {
			log.Println(tweet.Text)
		}
	}()
	return tweets
}

// send incoming tweets to one or more channels; slow client can block
func broadcastTweets(tweets chan *twitterstream.Tweet, clients ...chan *twitterstream.Tweet) {
	for tweet := range tweets {
		for _, client := range clients {
			client <- tweet
		}
	}
	// close client chan when finished
	for _, client := range clients {
		close(client)
	}
}

// setup streaming connection and tweet chan, reconnect on errors
func connectStream(keywords []string, quit chan bool) chan *twitterstream.Tweet {
	client := twitterstream.NewClient(consumerKey, consumerSecret, accessToken, accessSecret)
	tweets := make(chan *twitterstream.Tweet, 100)

	go func() {
		wait := 1 // wait 1 second on connection error
		for {
			select {
			case <-quit:
				close(tweets)
				return
			default:
				conn, err := client.Track(keywords...)
				if err != nil {
					log.Println("error connecting to streaming api: ", err)
					wait = wait << 1 // exp. backoff

					if wait > 300 {
						log.Printf("wait too long to reconnect, quitting...")
						close(quit)
						return
					}

					log.Printf("waiting %d seconds before reconnect", wait)
					time.Sleep(time.Duration(wait) * time.Second)
					continue
				} else {
					wait = 1 // reset wait on successful connection
				}
				decodeTweets(tweets, conn, quit)
			}
		}
	}()
	return tweets
}

// read tweets from stream, send each to tweets chan, on quit signal, close connection and return
func decodeTweets(tweets chan *twitterstream.Tweet, conn *twitterstream.Connection, quit chan bool) {
	for {
		select {
		case <-quit:
			conn.Close()
			return
		default:
			if tweet, err := conn.Next(); err == nil {
				tweets <- tweet
			} else {
				log.Println("error decoding tweet: ", err)
				conn.Close()
				return
			}
		}
	}
}

// read keywords from a file, specified one per line; twitter only allows 400
// keywords, returned slice will be truncated
func readKeywordFile(filename string) error {
	words := make([]string, 0)

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	r := bufio.NewReader(f)

	for {
		line, _, err := r.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		words = append(words, string(line))
	}
	// truncate to 400
	if len(words) > 400 {
		words = words[:400]
	}
	keywords = words
	return nil
}

func readKeywordCLI(words string) {
	keywords = strings.Split(words, ",")
}

func readTwitterConfig(filename string) error {
	var err error
	c, err := config.ReadDefault(filename)

	consumerKey, _ = c.String("twitter", "consumer-key")
	consumerSecret, _ = c.String("twitter", "consumer-secret")
	accessToken, _ = c.String("twitter", "access-token")
	accessSecret, _ = c.String("twitter", "access-secret")

	return err
}

func checkConfig() {
	if consumerKey == "" {
		log.Fatal("missing consumer_key")
	}

	if consumerSecret == "" {
		log.Fatal("missing consumer_secret")
	}

	if accessToken == "" {
		log.Fatal("missing access_token")
	}

	if accessSecret == "" {
		log.Fatal("missing access_secret")
	}

	if len(keywords) == 0 {
		log.Fatal("no keywords specified")
	}
}
