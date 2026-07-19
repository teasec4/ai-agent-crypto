import 'package:flutterapp/Session/service/domain/response.dart';

/// SSE events emitted by the streaming endpoint.
sealed class SseEvent {}

class SseConnected extends SseEvent {
  final String? clientId;
  final String? writerClientId;
  final WriterRequest? request;
  SseConnected({this.clientId, this.writerClientId, this.request});
}

class SseMessage extends SseEvent {
  final String? clientId;
  final String role;
  final String content;
  SseMessage({this.clientId, required this.role, required this.content});
}

class SseThinking extends SseEvent {}

class SseToolStart extends SseEvent {
  final String? clientId;
  final String tool;
  final Map<String, dynamic>? args;
  SseToolStart({this.clientId, required this.tool, this.args});
}

class SseToolDone extends SseEvent {
  final String? clientId;
  final String tool;
  final String? result;
  SseToolDone({this.clientId, required this.tool, this.result});
}

class SseToolError extends SseEvent {
  final String? clientId;
  final String tool;
  final String? error;
  SseToolError({this.clientId, required this.tool, this.error});
}

class SseApprovalRequired extends SseEvent {
  final String? clientId;
  final String tool;
  final PendingAction? action;

  SseApprovalRequired({this.clientId, required this.tool, this.action});
}

class SseDone extends SseEvent {
  final String? clientId;
  final String answer;
  SseDone({this.clientId, required this.answer});
}

class SseClose extends SseEvent {}

class SseWriterChanged extends SseEvent {
  final String? clientId;
  final String? writerClientId;
  SseWriterChanged({this.clientId, this.writerClientId});
}

class SseWriterRequestCreated extends SseEvent {
  final String? clientId;
  final String? writerClientId;
  final WriterRequest? request;
  SseWriterRequestCreated({this.clientId, this.writerClientId, this.request});
}

class SseWriterRequestResolved extends SseEvent {
  final String? clientId;
  final String? writerClientId;
  final bool? approved;
  SseWriterRequestResolved({this.clientId, this.writerClientId, this.approved});
}

class SseWorkspaceChanged extends SseEvent {}

class PendingAction {
  final String? id;
  final String tool;
  final String? risk;
  final String? summary;
  final String? preview;
  final Map<String, dynamic>? args;
  final DateTime? createdAt;

  const PendingAction({
    this.id,
    required this.tool,
    this.risk,
    this.summary,
    this.preview,
    this.args,
    this.createdAt,
  });

  factory PendingAction.fromJson(Map<String, dynamic> json) {
    return PendingAction(
      id: json['id'] as String?,
      tool: json['tool'] as String? ?? '',
      risk: json['risk'] as String?,
      summary: json['summary'] as String?,
      preview: json['preview'] as String?,
      args: json['args'] as Map<String, dynamic>?,
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? ''),
    );
  }
}
