package smscsim;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;

class Utils {

  private Utils() {}

  static int parseInt(byte[] bytes, int off) {
    int len = bytes.length;
    if (len < off + 4) {
      throw new RuntimeException("not enough bytes for int parsing");
    }
    int i1 = bytes[off] & 0xff;
    int i2 = bytes[off+1] & 0xff;
    int i3 = bytes[off+2] & 0xff;
    int i4 = bytes[off+3] & 0xff;
    return (i1 << 24) | (i2 << 16) | (i3 << 8) | i4;
  }

  static String parseCStr(ByteBuffer buf) {
    int start = buf.position();
    while(buf.hasRemaining() && buf.get() != 0) {} // search for null terminator
    int end = buf.position();
    return new String(Arrays.copyOfRange(buf.array(), start, end - 1), StandardCharsets.US_ASCII);
  }
}
