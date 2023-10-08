import 'dart:typed_data';

enum _LogLevel {
  info,
  error
}

void _log(_LogLevel lvl, String msg) {
  var time = DateTime.now().toString();
  print("$time [${lvl.name}] $msg");
}

void logInfo(String msg) {
  _log(_LogLevel.info, msg);
}

void logError(String msg) {
  _log(_LogLevel.error, msg);
}

extension PduParsing on ByteData {

  int setCString(int offset, String? s) {
    if (s != null) {
      for (var c in s.codeUnits) {
        setUint8(offset++, c);
      }
    }
    setUint8(offset++, 0); // null terminator
    return offset;
  }

  int setBytes(int offset, ByteData? src) {
    if (src != null) {
      for (int i = 0; i < src.lengthInBytes; i++) {
        setUint8(offset++, src.getUint8(i));
      }
    }
    return offset;
  }

  (String, int) getCString(int offset) {
    int nextOffset = offset;
    for (int i = offset; i < lengthInBytes; i++) {
      // search for null terminator
      if (getUint8(i) == 0) {
        nextOffset = i;
        break;
      }
    }
    var systemId = String.fromCharCodes(Uint8List.view(buffer), offset, nextOffset);
    return (systemId, nextOffset + 1);
  }
}

extension PduSerialization on String {

  ByteData asBytes() {
    var bytes = ByteData(length);
    int i = 0;
    for (final c  in codeUnits) {
      bytes.setUint8(i++, c);
    }
    return bytes;
  }

  ByteData asCStringBytes() {
    var bytes = ByteData(length + 1);
    int i = 0;
    for (final c in codeUnits) {
      bytes.setUint8(i++, c);
    }
    bytes.setUint8(i++, 0); // null terminator
    return bytes;
  }

  ByteData asUCS2Bytes() {
    var runes = this.runes;
    var bytes = ByteData(runes.length*2);
    int offset = 0;
    for (var r in runes) {
      var c = r <= 65536 ? r.toUnsigned(16) : 63; // 63 for ? sign
      bytes.setUint16(offset, c);
      offset += 2;
    }
    return bytes;
  }
}
