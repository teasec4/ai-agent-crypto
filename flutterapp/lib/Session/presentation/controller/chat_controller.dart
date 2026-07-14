import 'package:flutter/foundation.dart';
import 'package:flutterapp/Session/service/api_service.dart';
import 'package:flutterapp/Session/service/domain/chat_message.dart';
import 'package:flutterapp/Session/service/domain/sse_event.dart';

class ChatController extends ChangeNotifier {
  final String sessionId;

  ChatController({required this.sessionId});

  final List<ChatMessage> _messages = [];
  bool _loading = false;
  PendingAction? _pendingApproval;

  List<ChatMessage> get messages => List.unmodifiable(_messages);
  bool get loading => _loading;
  PendingAction? get pendingApproval => _pendingApproval;
  bool get canSend => !_loading && _pendingApproval == null;

  Future<void> send(String text) async {
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

  void _add(String role, String text) {
    _messages.add(ChatMessage(role: role, text: text));
    notifyListeners();
  }
}
