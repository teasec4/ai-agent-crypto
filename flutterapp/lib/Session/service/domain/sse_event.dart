/// SSE events emitted by the streaming endpoint.
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
  final PendingAction? action;

  SseApprovalRequired({required this.tool, this.action});
}

class SseDone extends SseEvent {
  final String answer;
  SseDone({required this.answer});
}

class SseClose extends SseEvent {}

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
