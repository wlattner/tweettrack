package main

import "html/template"

var (
	indexPage = template.Must(template.New("index").Parse(`
			<html>
			  <head>
			    <title>Tweet Stream</title>
			    <script type="text/javascript" src="//ajax.googleapis.com/ajax/libs/jquery/2.0.3/jquery.min.js"></script>
			    <script type="text/javascript">
			      var conn = new WebSocket("ws://{{.}}/ws");
			      conn.onclose = function(event) {
			        console.log('closed');
			      };
			      conn.onmessage = function(event) {
			        var tweet = JSON.parse(event.data)
			        $('#messages').prepend("<p>" + tweet.text + "</p>")
			        if ($('#messages>p').length > 100) {
			          $('#messages>p').last().remove()
			        }
			      };
			    </script>
			  </head>
			  <body>
			    <h2>Tweets</h2>
			    <div id="messages">
			    </div>
			  </body>
			</html>
		`))
)
