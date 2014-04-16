tweettrack
==========

This app is a simple experiment with the [Twitter Streaming API](https://dev.twitter.com/docs/api/streaming). *tweettrack* will track the user-supplied keywords (up to 400). Incoming tweets will be written to a file (tweets.json by default) and streamed to websocket clients.

Running
=======

Ensure your Twitter API keys are in `twitter_keys.cfg`. To run the app with the example keywords:
    
    ./tweettrack --keyword_file keywords.txt --port 3000

Then visit [http://localhost:3000](http://localhost:3000) in your browser.

To see additional configuration options:

	./tweettrack --help