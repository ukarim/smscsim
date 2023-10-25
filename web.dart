import 'dart:collection';
import 'dart:io';
import 'dart:convert';

import 'util.dart';
import 'smsc.dart';

class WebServer {

  final SmscServer _smsc;

  WebServer(this._smsc);

  void start(int port) async {
    logInfo("Starting web server on port $port");
    var server = await HttpServer.bind(InternetAddress.anyIPv4, port);
    server.forEach((HttpRequest req) {
      switch(req.method) {
        case 'POST': _handlePost(req);
        case 'GET': _handleGet(req);
        default: _reject(req);
      }
    });
  }

  void _reject(HttpRequest req) {
    var resp = req.response;
    resp.statusCode = HttpStatus.methodNotAllowed;
    resp.write('405 Method Not Allowed');
  }

  void _handlePost(HttpRequest req) {
    utf8.decodeStream(req).then((reqBody) {
      var data = _parsePostData(reqBody);
      var sender = data['sender'] ?? '';
      var recipient = data['recipient'] ?? '';
      var message = data['message'] ?? '';
      var systemId = data['system_id'] ?? '';

      var (res, err) = _smsc.sendMoMessage(sender, recipient, message, systemId);

      var respParams = LinkedHashMap<String, String?>();
      if (err) {
        respParams['error'] = res;
      } else {
        respParams['message'] = 'MO message was successfully sent';
      }
      respParams['sender'] = sender;
      respParams['recipient'] = recipient;

      var respUri = Uri.http(req.uri.authority, req.uri.path, respParams);
      var resp = req.response;
      resp.statusCode = HttpStatus.seeOther;
      resp.headers.add('Location', respUri.toString());
      resp.close();
    });
  }

  Map<String, String?> _parsePostData(String reqBody) {
    var data = LinkedHashMap<String, String?>();
    reqBody.split("&").forEach((pair) {
      var p = pair.split("=");
      data[p[0]] = Uri.decodeQueryComponent(p[1]);
    });
    return data;
  }

  void _handleGet(HttpRequest req) {
    // extract our query params
    var queryParams = req.requestedUri.queryParameters;
    var err = queryParams['error'];
    var msg = queryParams['message'];
    var sender = queryParams['sender'] ?? '';
    var recipient = queryParams['recipient'] ?? '';

    // prepare template variables
    var errFlag = false;
    var htmlMsg = '';
    if (msg != null && msg.isNotEmpty) {
      htmlMsg = msg;
    } else if (err != null && err.isNotEmpty) {
      errFlag = true;
      htmlMsg = err;
    }

    var html = _buildFormHtml(sender, recipient, htmlMsg, errFlag);

    // write response
    var resp = req.response;
    resp.statusCode = HttpStatus.ok;
    resp.headers.contentType = ContentType.html;
    resp.write(html);
    resp.close();
  }

  String _buildSystemIdOptions() {
    var buf = StringBuffer();
    _smsc.boundSystemIds().forEach((systemId) {
      var option = "<option value=\"${systemId}\">${systemId}</option>";
      buf.write(option);
    });
    return buf.toString();
  }

  String _buildFormHtml(String sender, String recipient, String msg, bool err) {
    var htmlClass = err ? 'error' : 'message';
    var html = """
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
          <input id="sender" type="text" name="sender" placeholder="sender" value="${sender}">
        </p>
        <p>
          <label for="recipient">Recipient (short number)</label>
          <input id="recipient" type="text" name="recipient" placeholder="recipient" value="${recipient}">
        </p>
        <p>
          <label for="system_id">System ID</label>
          <select id="system_id" name="system_id">
            ${_buildSystemIdOptions()}
          </select>
        </p>
        <p>
          <label for="short_message">Short message</label>
          <textarea id="short_message" name="message" maxlength="70" placeholder="Short message..."></textarea>
        </p>
        <p>
          <input type="submit" value="Submit">
        </p>
        <p id="${htmlClass}">${msg}</p>
      </form>
      </div>
      </body>
      </html>
    """;
    return html;
  }
}