package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"text/template"
)

const WEB_PAGE_TPL = `
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>smscsim web page</title>
  <style>
    html, body {
      padding: 0;
      margin: 0;
      font-size: 20px;
      font-family: sans-serif;
      background: #f0f0f0;
    }
    #container {
      margin: 40px auto;
      width: 560px;
      padding: 10px 40px;
      border-radius: 6px;
      box-shadow: 0 0 7px #dfdfdf;
      background: #fff;
    }
    #title {
      color: #3585f7;
      font-weight: bold;
      text-transform: uppercase;
      font-size: 24px;
    }
    form {
      margin: 20px auto;
      color: #394045;
      padding: 10px;
      width: 400px;
    }
    input, label, textarea {
      display: block;
      box-sizing: border-box;
      width: 100%;
      border: none;
      color: #657c89;
    }
    label {
      text-transform: uppercase;
      color: #657c89;
      font-size: 14px;
      font-weight: bold;
      padding: 0;
    }
    input, textarea {
      background: #f0f0f0;
      font-size: 20px;
      padding: 10px;
      margin: 5px 0 20px 0;
      border-radius: 3px;
    }
    textarea {
      resize: vertical;
    }
    input[type="submit"] {
      font-weight: bold;
      font-size: 16px;
      color: #fff;
      text-transform: uppercase;
      background: #3585f7;
    }
  </style>
</head>
<body>
<div id="container">
<form action="/" method="POST">
  <p id="title">Send MO message</p>
  <p>
    <label for="sender">Sender (MSISDN)</label>
    <input id="sender" type="text" name="sender" placeholder="sender">
  </p>
  <p>
    <label for="recipient">Recipient (short number)</label>
    <input id="recipient" type="text" name="recipient" placeholder="recipient">
  </p>
  <p>
    <label for="system_id">System ID</label>
    <select id="system_id" name="system_id">
    {{ range $systemId := .SystemIds }}
      <option value="{{ $systemId }}">{{ $systemId }}</option>
    {{ end }}
    </select>
  </p>
  <p>
    <label for="message">Short message</label>
    <textarea id="message" name="message" maxlength="70" placeholder="Short message..."></textarea>
  </p>
  <p>
    <input type="submit" value="Submit">
  </p>
</form>
</div>
</body>
</html>
`

type WebServer struct {
	Smsc Smsc
}

type TplVars struct {
	SystemIds []string
}

func NewWebServer(smsc Smsc) WebServer {
	return WebServer{smsc}
}

func (webServer *WebServer) Start(port int, wg sync.WaitGroup) {
	defer wg.Done()

	http.HandleFunc("/", webHandler(&webServer.Smsc))
	log.Println("Starting web server on port", port)
	log.Fatal(http.ListenAndServe(fmt.Sprint(":", port), nil))
}

func webHandler(smsc *Smsc) func(http.ResponseWriter, *http.Request) {
	if smsc == nil {
		log.Fatal("nil Smsc provided to web handler")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				log.Printf("Cannot parse POST params due [%v]", err)
			} else {
				// parse params
				params := r.Form
				sender := params.Get("sender")
				recipient := params.Get("recipient")
				message := params.Get("message")
				systemId := params.Get("system_id")
				// send MO
				smsc.SendMoMessage(sender, recipient, message, systemId)
				http.Redirect(w, r, "/", http.StatusSeeOther)
			}
		} else {
			tpl, err := template.New("webpage").Parse(WEB_PAGE_TPL)
			if err != nil {
				log.Fatal("Cannot parse template of the web page")
			}
			systemIds := smsc.BoundSystemIds()
			tpl.Execute(w, TplVars{systemIds})
		}
	}
}
