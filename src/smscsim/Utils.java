package smscsim;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;

class Utils {

  private static final int UDH_MAX_CONTENT_LENGTH = 134;
  private static final int MAX_SHORT_MSG_LENGTH = 140;

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

  public static List<byte[]> toUdhParts(byte[] bytes) {
    if (bytes.length <= MAX_SHORT_MSG_LENGTH) {
      // no need to split
      return Collections.singletonList(bytes);
    }
    int len = bytes.length;
    int count = (int) Math.ceil(((double) len)/ UDH_MAX_CONTENT_LENGTH);
    var partsList = new ArrayList<byte[]>(count);

    for (int i = 0; i < count; i++) {
      int startIdx = i * UDH_MAX_CONTENT_LENGTH;
      int endIdx = Math.min(startIdx + UDH_MAX_CONTENT_LENGTH, len);
      int partLen = endIdx - startIdx;
      byte[] udhPart = new byte[partLen + 6]; // plus 6 for udh headers
      udhPart[0] = 0x05;
      udhPart[1] = 0x00;
      udhPart[2] = 0x03;
      udhPart[3] = 0x01; // maybe accept id as method argument?
      udhPart[4] = (byte) count;
      udhPart[5] = (byte) (i + 1);
      System.arraycopy(bytes, startIdx, udhPart, 6, partLen);
      partsList.add(udhPart);
    }
    return partsList;
  }
}
