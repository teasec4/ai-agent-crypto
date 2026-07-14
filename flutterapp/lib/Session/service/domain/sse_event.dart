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
  SseApprovalRequired({required this.tool});
}

class SseDone extends SseEvent {
  final String answer;
  SseDone({required this.answer});
}

class SseClose extends SseEvent {}
