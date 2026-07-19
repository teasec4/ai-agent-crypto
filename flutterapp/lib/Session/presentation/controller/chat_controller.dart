import 'package:flutter/foundation.dart';
import 'package:flutterapp/Session/service/domain/response.dart';
import 'package:flutterapp/Session/service/api_service.dart';
import 'package:flutterapp/Session/service/domain/chat_message.dart';
import 'package:flutterapp/Session/service/domain/sse_event.dart';

class ChatController extends ChangeNotifier {
  final String sessionId;
  late final Future<void> _initFuture;

  ChatController({required this.sessionId}) {
    _initFuture = _init();
  }

  final List<ChatMessage> _messages = [];
  bool _loading = false;
  bool _loaded = false;
  String? _loadError;
  PendingAction? _pendingApproval;
  String? _workspace;

  List<ChatMessage> get messages => List.unmodifiable(_messages);
  bool get loading => _loading;
  bool get loaded => _loaded;
  String? get loadError => _loadError;
  PendingAction? get pendingApproval => _pendingApproval;
  String get title {
    final workspace = _workspace?.trim();
    if (workspace == null || workspace.isEmpty) return 'Project';
    final parts = workspace
        .replaceAll('\\', '/')
        .split('/')
        .where((part) => part.isNotEmpty)
        .toList();
    return parts.isEmpty ? workspace : parts.last;
  }

  String? get workspace => _workspace;
  bool get canSend =>
      _loaded && _loadError == null && !_loading && _pendingApproval == null;

  Future<void> ensureLoaded() => _initFuture;

  Future<void> _init() async {
    try {
      final detail = await ApiService.getSessionDetail(sessionId);
      _applySessionDetail(detail);
    } catch (e) {
      _loadError = e.toString();
    } finally {
      _loaded = true;
      notifyListeners();
    }
  }

  Future<void> send(String text) async {
    await _initFuture;
    final message = text.trim();
    if (message.isEmpty || !canSend) return;

    _messages.add(ChatMessage(role: 'user', text: message));
    _loading = true;
    _pendingApproval = null;
    notifyListeners();

    try {
      await ApiService.stream(sessionId, message, _onSseEvent);
    } catch (e) {
      _add('assistant', 'Error: $e');
    } finally {
      _loading = false;
      notifyListeners();
    }
  }

  Future<void> submitApproval(bool approved) async {
    await _initFuture;
    final action = _pendingApproval;
    if (action == null) return;

    _pendingApproval = null;
    _loading = true;
    _messages.add(
      ChatMessage(
        role: 'tool',
        text: approved ? 'Approval sent.' : 'Rejection sent.',
      ),
    );
    notifyListeners();

    try {
      if (approved) {
        await ApiService.approve(sessionId);
      } else {
        await ApiService.reject(sessionId);
      }
    } catch (e) {
      _pendingApproval = action;
      _loading = false;
      _messages.add(ChatMessage(role: 'assistant', text: 'Approval error: $e'));
      notifyListeners();
      rethrow;
    }
  }

  void _onSseEvent(SseEvent event) {
    switch (event) {
      case SseThinking():
        break;

      case SseToolStart(:final tool, :final args):
        _add('tool', '-> $tool${args != null ? ' $args' : ''}');

      case SseToolDone(:final tool, :final result):
        final preview = (result != null && result.length > 120)
            ? '${result.substring(0, 120)}...'
            : (result ?? '');
        _add('tool', 'done $tool: $preview');

      case SseToolError(:final tool, :final error):
        _add('tool', 'error $tool: $error');

      case SseApprovalRequired(:final tool):
        final action = event.action ?? PendingAction(tool: tool);
        _pendingApproval = action;
        _loading = false;
        _messages.add(
          ChatMessage(
            role: 'assistant',
            text: action.summary ?? '"${action.tool}" requires approval',
          ),
        );
        notifyListeners();

      case SseDone(:final answer):
        _pendingApproval = null;
        _add('assistant', answer);

      case SseClose():
        _pendingApproval = null;
        notifyListeners();
    }
  }

  void _applySessionDetail(SessionDetailResponse detail) {
    _workspace = detail.workspace;
    _messages
      ..clear()
      ..addAll(
        detail.messages
            .where((message) => message.role != 'system')
            .where(
              (message) => !message.content.startsWith('Tool observation: '),
            )
            .map(
              (message) =>
                  ChatMessage(role: message.role, text: message.content),
            ),
      );
  }

  void _add(String role, String text) {
    _messages.add(ChatMessage(role: role, text: text));
    notifyListeners();
  }
}
