package smscsim;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.IOException;
import java.io.InputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.Executors;

import static java.lang.System.Logger.Level.ERROR;
import static java.lang.System.Logger.Level.INFO;
import static java.net.URLDecoder.decode;
import static java.net.URLEncoder.encode;

class WebServer implements Runnable {

  private final System.Logger logger = System.getLogger("WebServer");
  private final int port;
  private final SmscServer smscServer;

  WebServer(int port, SmscServer smscServer) {
    this.port = port;
    this.smscServer = smscServer;
  }

  public void run() {
    try {
      logger.log(INFO, "Starting web server on port " + port);
      var server = HttpServer.create(new InetSocketAddress("0.0.0.0", port), 0);
      server.setExecutor(Executors.newVirtualThreadPerTaskExecutor());
      server.createContext("/", exchange -> {
        try {
          String reqMethod = exchange.getRequestMethod();
          switch (reqMethod) {
            case "POST" -> handlePost(exchange);
            case "GET" -> handleGet(exchange);
            default -> handleUnsupportedMethod(exchange);
          }
        } catch (Exception e) {
          logger.log(ERROR, "error handling request", e);
          sendResponse(exchange, 500, "internal server error");
        }
      });
      server.start();
    } catch (Exception e) {
      logger.log(ERROR, "Error starting WebServer", e);
      System.exit(1);
    }
  }

  private void handlePost(HttpExchange exchange) throws IOException {
    Map<String, String> reqParams = readWwwEncodedBody(exchange);
    String sender = reqParams.getOrDefault("sender", "");
    String recipient = reqParams.getOrDefault("recipient", "");
    String message = reqParams.getOrDefault("message", "");
    String systemId = reqParams.getOrDefault("system_id", "");

    Optional<String> result = smscServer.sendMoMessage(sender, recipient, message, systemId);
    var params = new HashMap<String, String>();
    if (result.isEmpty()) {
      params.put("message", "MO message was successfully sent");
    } else {
      params.put("error", result.get());
    }
    params.put("sender", sender);
    params.put("recipient", recipient);

    String location = "/?" + buildUrlParams(params);
    exchange.getResponseHeaders().add("Location", location);
    sendResponse(exchange, 303, String.format("Redirect <a href='%s'>to</a>", location));
  }

  private void handleGet(HttpExchange exchange) throws IOException {
    Map<String, String> queryParams = parseQuery(exchange.getRequestURI().getQuery());
    String tmpl = """
        <!doctype html>
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
            select {
              min-width: 200px;
            }
            input[type="submit"] {
              font-weight: bold;
              font-size: 16px;
              color: #fff;
              text-transform: uppercase;
              background: #3585f7;
            }
            #message {
              color: #009688;
            }
            #error {
              color: #f44336;
            }
          </style>
        </head>
        <body>
        <div id="container">
        <form action="/" method="POST">
          <p id="title">Send MO message</p>
          <p>
            <label for="sender">Sender (MSISDN)</label>
            <input id="sender" type="text" name="sender" placeholder="sender" value="{sender}">
          </p>
          <p>
            <label for="recipient">Recipient (short number)</label>
            <input id="recipient" type="text" name="recipient" placeholder="recipient" value="{recipient}">
          </p>
          <p>
            <label for="system_id">System ID</label>
            <select id="system_id" name="system_id">
              {options}
            </select>
          </p>
          <p>
            <label for="short_message">Short message</label>
            <textarea id="short_message" name="message" placeholder="Short message..."></textarea>
          </p>
          <p>
            <input type="submit" value="Submit">
          </p>
          <p id="{info_class}">{info_message}</p>
        </form>
        </div>
        </body>
        </html>
        """;

    String error = queryParams.getOrDefault("error", "");
    String message = queryParams.getOrDefault("message", "");
    String msgBoxClass;
    String msgBoxMsg;
    if (!message.isBlank()) {
      msgBoxClass = "message";
      msgBoxMsg = message;
    } else {
      msgBoxClass = "error";
      msgBoxMsg = error;
    }
    tmpl = tmpl.replace("{sender}", queryParams.getOrDefault("sender", ""));
    tmpl = tmpl.replace("{recipient}", queryParams.getOrDefault("recipient", ""));
    tmpl = tmpl.replace("{options}", buildOptionsHtml());
    tmpl = tmpl.replace("{info_class}", msgBoxClass);
    var html = tmpl.replace("{info_message}", msgBoxMsg);
    sendResponse(exchange, 200, html);
  }

  private void handleUnsupportedMethod(HttpExchange exchange) throws IOException {
    byte[] resp = "unsupported method".getBytes(StandardCharsets.UTF_8);
    exchange.sendResponseHeaders(415, resp.length);
    exchange.getResponseBody().write(resp);
  }

  private Map<String, String> parseQuery(String query) {
    if (query == null || query.isBlank()) {
      return Collections.emptyMap();
    }
    if (query.startsWith("?")) {
      query = query.substring(1);
    }
    var queryParams = new HashMap<String, String>();
    for (String pair : query.split("&")) {
      String[] kv = pair.split("=", 2);
      queryParams.put(kv[0], kv[1]);
    }
    return queryParams;
  }

  private String buildOptionsHtml() {
    var buf = new StringBuilder();
    for (String systemId : smscServer.boundSystemIds()) {
      buf.append(String.format("<option value='%s'>%s</option>", systemId, systemId));
    }
    return buf.toString();
  }

  private Map<String, String> readWwwEncodedBody(HttpExchange exchange) throws IOException {
    String reqBody;
    try (InputStream in = exchange.getRequestBody()) {
      reqBody = new String(in.readAllBytes(), StandardCharsets.UTF_8);
    }
    var res = new HashMap<String, String>();
    for (String kv : reqBody.split("&")) {
      String[] pair = kv.split("=", 2);
      res.put(decode(pair[0], StandardCharsets.UTF_8), decode(pair[1], StandardCharsets.UTF_8));
    }
    return res;
  }

  private String buildUrlParams(Map<String, String> params) {
    var buf = new StringBuilder();
    for (Map.Entry<String, String> e : params.entrySet()) {
      buf
          .append(encode(e.getKey(), StandardCharsets.UTF_8))
          .append("=")
          .append(encode(e.getValue(), StandardCharsets.UTF_8).replaceAll("\\+", "%20"))
          .append("&");
    }
    return buf.toString();
  }

  private void sendResponse(HttpExchange exchange, int sts, String body) throws IOException {
    byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
    exchange.sendResponseHeaders(sts, bytes.length);
    exchange.getResponseBody().write(bytes);
    exchange.close();
  }
}
