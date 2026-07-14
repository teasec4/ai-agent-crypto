import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutterapp/Session/service/domain/request.dart';
import 'package:flutterapp/Session/service/domain/response.dart';

/// SSE event received from the stream endpoint.
sealed class SseEvent {}

class SseThinking extends SseEvent {}

class SseToolStart extends SseEvent {
  final String tool;
  final Map<String, dynamic>? args;
  SseToolStart({required this.tool, this.args});
}

class SseToolDone extends SseEvent {
  final String tool;
  final String? result;
  SseToolDone({required this.tool, this.result});
}

class SseToolError extends SseEvent {
  final String tool;
  final String? error;
  SseToolError({required this.tool, this.error});
}

class SseApprovalRequired extends SseEvent {
  final String tool;
  SseApprovalRequired({required this.tool});
}

class SseDone extends SseEvent {
  final String answer;
  SseDone({required this.answer});
}

class SseClose extends SseEvent {}

class ApiService {
  static const _host = 'http://localhost:8080';

  /// Create a new session and return its ID.
  static Future<String> createSession() async {
    final client = HttpClient();
    try {
      final req = await client.postUrl(Uri.parse('$_host/sessions'));
      final res = await req.close();
      final body = await res.transform(utf8.decoder).join();
      return jsonDecode(body)['sessionId'] as String;
    } finally {
      client.close();
    }
  }

  /// Set the workspace directory for a session.
  static Future<void> setWorkspace(String sessionId, String path) async {
    final client = HttpClient();
    try {
      final req = await client.postUrl(
        Uri.parse('$_host/sessions/$sessionId/workspace'),
      );
      req.headers.contentType = ContentType.json;
      req.write(jsonEncode({'path': path}));
      await req.close();
    } finally {
      client.close();
    }
  }

  /// Send a message via POST /ask (non-streaming, returns final answer).
  static Future<AskResponse> ask(String message, {String? sessionId}) async {
    final client = HttpClient();
    try {
      final req = await client.postUrl(Uri.parse('$_host/ask'));
      req.headers.contentType = ContentType.json;
      req.write(jsonEncode(AskRequest(sessionId: sessionId, message: message).toJson()));
      final res = await req.close();
      final body = await res.transform(utf8.decoder).join();
      return AskResponse.fromJson(jsonDecode(body) as Map<String, dynamic>);
    } finally {
      client.close();
    }
  }

  /// Stream a message via POST /sessions/{sessionId}/stream.
  /// [onEvent] is called for each SSE event as it arrives.
  static Future<void> stream(
    String sessionId,
    String message,
    void Function(SseEvent event) onEvent,
  ) async {
    final client = HttpClient();
    try {
      final req = await client.postUrl(
        Uri.parse('$_host/sessions/$sessionId/stream'),
      );
      req.headers.contentType = ContentType.json;
      req.write(jsonEncode({'message': message}));

      final res = await req.close();
      final lines = res.transform(utf8.decoder).transform(const LineSplitter());

      await for (final line in lines) {
        if (!line.startsWith('data: ')) continue;

        final data = jsonDecode(line.substring(6)) as Map<String, dynamic>;
        final type = data['type'] as String?;

        switch (type) {
          case 'thinking':
            onEvent(SseThinking());
          case 'tool_start':
            onEvent(SseToolStart(
              tool: data['tool'] as String,
              args: data['args'] as Map<String, dynamic>?,
            ));
          case 'tool_done':
            onEvent(SseToolDone(
              tool: data['tool'] as String,
              result: data['result'] as String?,
            ));
          case 'tool_error':
            onEvent(SseToolError(
              tool: data['tool'] as String,
              error: data['error'] as String?,
            ));
          case 'approval_required':
            onEvent(SseApprovalRequired(tool: data['tool'] as String));
          case 'done':
            onEvent(SseDone(answer: data['answer'] as String));
          case 'close':
            onEvent(SseClose());
        }
      }
    } finally {
      client.close();
    }
  }

  /// Approve a pending action (for SSE approval flow).
  static Future<void> approve(String sessionId) async {
    final client = HttpClient();
    try {
      final req = await client.postUrl(
        Uri.parse('$_host/sessions/$sessionId/approve'),
      );
      await req.close();
    } finally {
      client.close();
    }
  }

  /// Reject a pending action (for SSE approval flow).
  static Future<void> reject(String sessionId) async {
    final client = HttpClient();
    try {
      final req = await client.postUrl(
        Uri.parse('$_host/sessions/$sessionId/reject'),
      );
      await req.close();
    } finally {
      client.close();
    }
  }
}
