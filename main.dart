import 'dart:io';

import 'smsc.dart';
import 'web.dart';

final int DEF_SMSC_PORT = 2775;
final int DEF_WEB_PORT = 12775;

void main() {
  var env = Platform.environment;
  var smscPort = parseInt(env['SMSC_PORT'], DEF_SMSC_PORT);
  var webPort = parseInt(env['WEB_PORT'], DEF_WEB_PORT);
  var failedSubmits = parseBool(env['FAILED_SUBMITS'], false);

  var smsc = SmscServer(failedSubmits);
  smsc.start(smscPort);
  WebServer(smsc).start(webPort);
}

int parseInt(String? n, int def) {
  return int.tryParse(n ?? "$def") ?? def;
}

bool parseBool(String? b, bool def) {
  return bool.tryParse(b ?? "$def") ?? def;
}
