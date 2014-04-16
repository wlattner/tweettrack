package main

import (
	"encoding/json"
	"log"
	"net/http"

	"code.google.com/p/go.net/websocket"

	"github.com/darkhelmet/twitterstream"
)

var subscribe chan client

type client struct {
	conn *websocket.Conn
	done chan bool
}

func wsHandler(ws *websocket.Conn) {
	done := make(chan bool)
	subscribe <- client{ws, done} // add socket to subscription list
	<-done                        // keep connection open
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	indexPage.Execute(w, r.Host)
}

func startServer(port string) chan *twitterstream.Tweet {
	tweets := make(chan *twitterstream.Tweet)
	subscribe = make(chan client)

	go broadcaster(subscribe, tweets)

	go func() {
		http.Handle("/ws", websocket.Handler(wsHandler))
		http.HandleFunc("/", indexHandler)
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()
	return tweets
}

func broadcaster(subscribe chan client, tweets chan *twitterstream.Tweet) {
	subscribers := make(map[*websocket.Conn]chan bool)
	for {
		select {
		case subscriber := <-subscribe:
			subscribers[subscriber.conn] = subscriber.done
		case tweet := <-tweets:
			encodedTweet, err := json.Marshal(tweet)
			if err != nil {
				continue
			}
			for conn, quit := range subscribers {
				if _, err := conn.Write(encodedTweet); err != nil {
					// drop subscriber on errors
					delete(subscribers, conn)
					close(quit)
				}
			}
		}
	}
}
