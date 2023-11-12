package smscsim;

import java.util.concurrent.Executors;

public class Main {

  public static void main(String[] args) {
    int webPort = intEnvVar("WEB_PORT", 12775);
    int smppPort = intEnvVar("SMSC_PORT", 2775);
    boolean failedSubmits = boolEnvVar("FAILED_SUBMITS", false);

    var smscServer = new SmscServer(smppPort, failedSubmits);
    var webServer = new WebServer(webPort, smscServer);

    try (var pool = Executors.newFixedThreadPool(2)) {
      pool.submit(smscServer);
      pool.submit(webServer);
    }
  }

  private static int intEnvVar(String name, int def) {
    String env = System.getenv(name);
    if (env == null || env.isBlank()) {
      return def;
    }
    int res;
    try {
      res = Integer.parseInt(env);
    } catch (Exception e) {
      res = def;
    }
    return res;
  }

  private static boolean boolEnvVar(String name, boolean def) {
    String env = System.getenv(name);
    if (env == null || env.isBlank()) {
      return def;
    }
    boolean res;
    try {
      res = Boolean.parseBoolean(env);
    } catch (Exception e) {
      res = def;
    }
    return res;
  }
}
