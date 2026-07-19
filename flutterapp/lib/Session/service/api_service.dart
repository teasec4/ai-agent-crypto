import 'package:flutter/foundation.dart';
import 'dart:async';
import 'dart:convert';

import 'package:flutterapp/Session/service/domain/response.dart';
import 'package:flutterapp/Session/service/domain/sse_event.dart';
import 'package:http/http.dart' as http;

class ApiException implements Exception {
  final String message;
  final int? statusCode;

  const ApiException(this.message, {this.statusCode});

  @override
  String toString() {
    if (statusCode == null) return message;
    return 'HTTP $statusCode: $message';
  }
}

class ApiService {
  static final _defaultBaseUrl = String.fromEnvironment(
    'AGENT_API_BASE_URL',
    defaultValue: _platformDefaultBaseUrl(),
  );
  static const requestTimeout = Duration(seconds: 20);

  static Uri baseUri = Uri.parse(_defaultBaseUrl);

  static http.Client? _clientOverride;

  static String _platformDefaultBaseUrl() {
    if (kIsWeb) return 'http://localhost:8080';
    switch (defaultTargetPlatform) {
      case TargetPlatform.android:
        return 'http://10.0.2.2:8080';
      case TargetPlatform.iOS:
      case TargetPlatform.macOS:
      case TargetPlatform.linux:
      case TargetPlatform.windows:
      case TargetPlatform.fuchsia:
        return 'http://localhost:8080';
    }
  }

  static void configureForTesting({Uri? baseUriOverride, http.Client? client}) {
    if (baseUriOverride != null) baseUri = baseUriOverride;
    _clientOverride = client;
  }

  static Uri setBaseUrl(String value) {
    final normalized = _normalizeBaseUrl(value);
    baseUri = normalized;
    return baseUri;
  }

  static http.Client _newClient() => _clientOverride ?? http.Client();

  static bool get _shouldCloseClient => _clientOverride == null;

  /// Create a new session and return its ID.
  static Future<String> createSession() async {
    final json = await _postJson('/sessions');
    final sessionId = json['sessionId'] as String?;
    if (sessionId == null || sessionId.isEmpty) {
      throw const ApiException('Server did not return a sessionId');
    }
    return sessionId;
  }

  /// List backend sessions. The backend returns them newest first.
  static Future<List<SessionSummary>> listSessions() async {
    final json = await _getJsonList('/sessions');
    return json
        .map((item) => SessionSummary.fromJson(item as Map<String, dynamic>))
        .toList();
  }

  /// Load one session with messages and workspace.
  static Future<SessionDetailResponse> getSessionDetail(
    String sessionId,
  ) async {
    final json = await _getJson('/sessions/$sessionId');
    return SessionDetailResponse.fromJson(json);
  }

  /// Set the workspace directory for a session. The path is resolved by the backend.
  static Future<void> setWorkspace(String sessionId, String path) async {
    await _postJson('/sessions/$sessionId/workspace', body: {'path': path});
  }

  /// List filesystem roots exposed by the backend for the project picker.
  static Future<List<WorkspaceRoot>> listWorkspaceRoots() async {
    final json = await _getJson('/workspace/roots');
    return (json['roots'] as List? ?? const [])
        .map((item) => WorkspaceRoot.fromJson(item as Map<String, dynamic>))
        .toList();
  }

  /// Browse a backend filesystem directory.
  static Future<WorkspaceBrowseResponse> browseWorkspace(String path) async {
    final json = await _getJson('/workspace/browse', query: {'path': path});
    return WorkspaceBrowseResponse.fromJson(json);
  }

  /// Stream a message via POST /sessions/{sessionId}/stream.
  /// [onEvent] is called for each SSE event as it arrives.
  static Future<void> stream(
    String sessionId,
    String message,
    void Function(SseEvent event) onEvent,
  ) async {
    final client = _newClient();
    try {
      final request =
          http.Request('POST', _resolve('/sessions/$sessionId/stream'))
            ..headers['Accept'] = 'text/event-stream'
            ..headers['Content-Type'] = 'application/json'
            ..body = jsonEncode({'message': message});

      final response = await client.send(request).timeout(requestTimeout);
      if (response.statusCode < 200 || response.statusCode >= 300) {
        final body = await response.stream.bytesToString();
        throw ApiException(
          _extractError(body),
          statusCode: response.statusCode,
        );
      }

      await _readSse(response.stream, onEvent);
    } finally {
      if (_shouldCloseClient) client.close();
    }
  }

  /// Approve a pending action (for SSE approval flow).
  static Future<void> approve(String sessionId) async {
    await _postJson('/sessions/$sessionId/approve');
  }

  /// Reject a pending action (for SSE approval flow).
  static Future<void> reject(String sessionId) async {
    await _postJson('/sessions/$sessionId/reject');
  }

  static Future<Map<String, dynamic>> _postJson(
    String path, {
    Map<String, dynamic>? body,
  }) async {
    final client = _newClient();
    try {
      final response = await client
          .post(
            _resolve(path),
            headers: {'Content-Type': 'application/json'},
            body: body == null ? null : jsonEncode(body),
          )
          .timeout(requestTimeout);

      final responseBody = response.body;
      if (response.statusCode < 200 || response.statusCode >= 300) {
        throw ApiException(
          _extractError(responseBody),
          statusCode: response.statusCode,
        );
      }
      if (responseBody.trim().isEmpty) return <String, dynamic>{};
      return jsonDecode(responseBody) as Map<String, dynamic>;
    } on TimeoutException {
      throw const ApiException('Request timed out. Is the agent API running?');
    } finally {
      if (_shouldCloseClient) client.close();
    }
  }

  static Future<List<dynamic>> _getJsonList(String path) async {
    final client = _newClient();
    try {
      final response = await client.get(_resolve(path)).timeout(requestTimeout);

      if (response.statusCode < 200 || response.statusCode >= 300) {
        throw ApiException(
          _extractError(response.body),
          statusCode: response.statusCode,
        );
      }
      if (response.body.trim().isEmpty) return <dynamic>[];
      final decoded = jsonDecode(response.body);
      if (decoded is List) return decoded;
      throw const ApiException('Expected a list response from server');
    } on TimeoutException {
      throw const ApiException('Request timed out. Is the agent API running?');
    } finally {
      if (_shouldCloseClient) client.close();
    }
  }

  static Future<Map<String, dynamic>> _getJson(
    String path, {
    Map<String, String>? query,
  }) async {
    final client = _newClient();
    try {
      final response = await client
          .get(_resolve(path, query: query))
          .timeout(requestTimeout);

      if (response.statusCode < 200 || response.statusCode >= 300) {
        throw ApiException(
          _extractError(response.body),
          statusCode: response.statusCode,
        );
      }
      if (response.body.trim().isEmpty) return <String, dynamic>{};
      final decoded = jsonDecode(response.body);
      if (decoded is Map<String, dynamic>) return decoded;
      throw const ApiException('Expected an object response from server');
    } on TimeoutException {
      throw const ApiException('Request timed out. Is the agent API running?');
    } finally {
      if (_shouldCloseClient) client.close();
    }
  }

  static Uri _resolve(String path, {Map<String, String>? query}) {
    final normalized = path.startsWith('/') ? path.substring(1) : path;
    return baseUri.resolveUri(Uri(path: normalized, queryParameters: query));
  }

  static Uri _normalizeBaseUrl(String value) {
    var raw = value.trim();
    if (raw.isEmpty) {
      throw const ApiException('Server URL is required');
    }
    if (!raw.contains('://')) {
      raw = 'http://$raw';
    }

    var uri = Uri.tryParse(raw);
    if (uri == null || uri.host.trim().isEmpty) {
      throw const ApiException('Enter a valid server URL or IP address');
    }
    if (uri.scheme != 'http' && uri.scheme != 'https') {
      throw const ApiException(
        'Server URL must start with http:// or https://',
      );
    }

    if (!uri.hasPort) {
      uri = uri.replace(port: 8080);
    }
    final path = uri.path.isEmpty ? '/' : uri.path;
    return uri.replace(path: path, query: null, fragment: null);
  }

  static String _extractError(String body) {
    if (body.trim().isEmpty) return 'Empty error response from server';
    try {
      final json = jsonDecode(body) as Map<String, dynamic>;
      return json['error'] as String? ?? body;
    } catch (_) {
      return body;
    }
  }

  static Future<void> _readSse(
    Stream<List<int>> stream,
    void Function(SseEvent event) onEvent,
  ) async {
    String? eventName;
    final dataLines = <String>[];
    final lines = stream
        .transform(utf8.decoder)
        .transform(const LineSplitter());

    Future<void> dispatch() async {
      if (eventName == null && dataLines.isEmpty) return;
      final data = dataLines.join('\n');
      _dispatchSseEvent(eventName, data, onEvent);
      eventName = null;
      dataLines.clear();
    }

    await for (final rawLine in lines) {
      final line = rawLine.trimRight();
      if (line.isEmpty) {
        await dispatch();
        continue;
      }
      if (line.startsWith(':')) continue;
      if (line.startsWith('event:')) {
        eventName = line.substring(6).trim();
        continue;
      }
      if (line.startsWith('data:')) {
        dataLines.add(line.substring(5).trimLeft());
      }
    }
    await dispatch();
  }

  static void _dispatchSseEvent(
    String? eventName,
    String data,
    void Function(SseEvent event) onEvent,
  ) {
    final json = data.isEmpty
        ? <String, dynamic>{}
        : jsonDecode(data) as Map<String, dynamic>;
    final type = json['type'] as String? ?? eventName;

    switch (type) {
      case 'thinking':
        onEvent(SseThinking());
      case 'tool_start':
        onEvent(
          SseToolStart(
            tool: json['tool'] as String? ?? '',
            args: json['args'] as Map<String, dynamic>?,
          ),
        );
      case 'tool_done':
        onEvent(
          SseToolDone(
            tool: json['tool'] as String? ?? '',
            result: json['result'] as String?,
          ),
        );
      case 'tool_error':
        onEvent(
          SseToolError(
            tool: json['tool'] as String? ?? '',
            error: json['error'] as String?,
          ),
        );
      case 'approval_required':
        final actionJson = json['action'];
        final action = actionJson is Map<String, dynamic>
            ? PendingAction.fromJson(actionJson)
            : null;
        onEvent(
          SseApprovalRequired(
            tool: json['tool'] as String? ?? action?.tool ?? '',
            action: action,
          ),
        );
      case 'done':
        onEvent(SseDone(answer: json['answer'] as String? ?? ''));
      case 'close':
        onEvent(SseClose());
    }
  }
}
