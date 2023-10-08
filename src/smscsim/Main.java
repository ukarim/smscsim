package smscsim;

public class Main {

  public static void main(String[] args) throws Exception {
    int webPort = intEnvVar("WEB_PORT", 12775);
    int smppPort = intEnvVar("SMSC_PORT", 2775);
    boolean failedSubmits = boolEnvVar("FAILED_SUBMITS", false);

    var smscServer = new SmscServer(smppPort, failedSubmits);
    smscServer.start();
    var webServer = new WebServer(webPort, smscServer);
    webServer.start();

    // wait for stopping
    smscServer.join();
    webServer.join();
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
