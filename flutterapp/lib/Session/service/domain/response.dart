class SessionSummary {
  final String id;
  final DateTime? createdAt;
  final DateTime? updatedAt;
  final int messageCount;
  final String? workspace;

  SessionSummary({
    required this.id,
    this.createdAt,
    this.updatedAt,
    required this.messageCount,
    this.workspace,
  });

  factory SessionSummary.fromJson(Map<String, dynamic> json) {
    return SessionSummary(
      id: json['id'] as String,
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? ''),
      updatedAt: DateTime.tryParse(json['updatedAt'] as String? ?? ''),
      messageCount: (json['messageCount'] as num?)?.toInt() ?? 0,
      workspace: json['workspace'] as String?,
    );
  }
}

class SessionDetailResponse {
  final String id;
  final String sessionId;
  final DateTime? createdAt;
  final DateTime? updatedAt;
  final int messageCount;
  final String? workspace;
  final List<SessionMessage> messages;

  SessionDetailResponse({
    required this.id,
    required this.sessionId,
    this.createdAt,
    this.updatedAt,
    required this.messageCount,
    this.workspace,
    required this.messages,
  });

  factory SessionDetailResponse.fromJson(Map<String, dynamic> json) {
    return SessionDetailResponse(
      id: json['id'] as String,
      sessionId: json['sessionId'] as String,
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? ''),
      updatedAt: DateTime.tryParse(json['updatedAt'] as String? ?? ''),
      messageCount: (json['messageCount'] as num?)?.toInt() ?? 0,
      workspace: json['workspace'] as String?,
      messages: (json['messages'] as List? ?? const [])
          .map((item) => SessionMessage.fromJson(item as Map<String, dynamic>))
          .toList(),
    );
  }
}

class SessionMessage {
  final String role;
  final String content;
  final List<dynamic>? toolCalls;
  final String? toolCallId;

  SessionMessage({
    required this.role,
    required this.content,
    this.toolCalls,
    this.toolCallId,
  });

  factory SessionMessage.fromJson(Map<String, dynamic> json) {
    return SessionMessage(
      role: json['role'] as String? ?? '',
      content: json['content'] as String? ?? '',
      toolCalls: json['tool_calls'] as List?,
      toolCallId: json['tool_call_id'] as String?,
    );
  }
}

class WorkspaceRoot {
  final String path;
  final String name;
  final String kind;

  WorkspaceRoot({required this.path, required this.name, required this.kind});

  factory WorkspaceRoot.fromJson(Map<String, dynamic> json) {
    return WorkspaceRoot(
      path: json['path'] as String,
      name: json['name'] as String? ?? '',
      kind: json['kind'] as String? ?? '',
    );
  }
}

class WorkspaceEntry {
  final String name;
  final String path;
  final bool isDir;

  WorkspaceEntry({required this.name, required this.path, required this.isDir});

  factory WorkspaceEntry.fromJson(Map<String, dynamic> json) {
    return WorkspaceEntry(
      name: json['name'] as String? ?? '',
      path: json['path'] as String? ?? '',
      isDir: json['isDir'] as bool? ?? false,
    );
  }
}

class WorkspaceBrowseResponse {
  final String path;
  final String? parentPath;
  final List<WorkspaceRoot> roots;
  final List<WorkspaceEntry> entries;

  WorkspaceBrowseResponse({
    required this.path,
    this.parentPath,
    required this.roots,
    required this.entries,
  });

  factory WorkspaceBrowseResponse.fromJson(Map<String, dynamic> json) {
    return WorkspaceBrowseResponse(
      path: json['path'] as String? ?? '',
      parentPath: json['parentPath'] as String?,
      roots: (json['roots'] as List? ?? const [])
          .map((item) => WorkspaceRoot.fromJson(item as Map<String, dynamic>))
          .toList(),
      entries: (json['entries'] as List? ?? const [])
          .map((item) => WorkspaceEntry.fromJson(item as Map<String, dynamic>))
          .toList(),
    );
  }
}
