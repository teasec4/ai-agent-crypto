import 'package:flutter_test/flutter_test.dart';
import 'package:flutterapp/Session/service/api_service.dart';

void main() {
  test('normalizes a host or IP into a usable api base url', () {
    final uri = ApiService.setBaseUrl('192.168.1.20');
    expect(uri.toString(), 'http://192.168.1.20:8080/');

    final url = ApiService.setBaseUrl('http://10.0.0.5:9090');
    expect(url.toString(), 'http://10.0.0.5:9090/');
  });
}
