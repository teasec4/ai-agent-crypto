class AskResponse {
  final String sessionId;
  final String answer;
  final int iterations;
  final String stoppedBy;
  final List<Trace>? trace;

  AskResponse({
    required this.sessionId,
    required this.answer,
    required this.iterations,
    required this.stoppedBy,
    this.trace,
  });

  factory AskResponse.fromJson(Map<String, dynamic> json) {
    return AskResponse(
      sessionId: json['sessionId'] as String,
      answer: json['answer'] as String,
      iterations: json['iterations'] as int,
      stoppedBy: json['stoppedBy'] as String,
      trace: (json['trace'] as List?)
          ?.map((t) => Trace.fromJson(t as Map<String, dynamic>))
          .toList(),
    );
  }
}

class Trace {
  final int index;
  final String outcome;
  final List<ToolEvent>? toolEvents;

  Trace({required this.index, required this.outcome, this.toolEvents});

  factory Trace.fromJson(Map<String, dynamic> json) {
    return Trace(
      index: json['index'] as int,
      outcome: json['outcome'] as String,
      toolEvents: (json['toolEvents'] as List?)
          ?.map((e) => ToolEvent.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }
}

class ToolEvent {
  final String tool;
  final Map<String, dynamic>? args;
  final int contextSize;

  ToolEvent({required this.tool, this.args, required this.contextSize});

  factory ToolEvent.fromJson(Map<String, dynamic> json) {
    return ToolEvent(
      tool: json['tool'] as String,
      args: json['args'] as Map<String, dynamic>?,
      contextSize: (json['contextSize'] as num?)?.toInt() ?? 0,
    );
  }
}
