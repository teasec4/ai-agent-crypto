import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutterapp/Session/service/domain/response.dart';
import 'package:flutterapp/Session/service/api_service.dart';
import 'package:flutterapp/Session/service/domain/chat_message.dart';
import 'package:flutterapp/Session/service/domain/sse_event.dart';

class ChatController extends ChangeNotifier {
  static final Map<String, String> _clientIdsBySession = {};

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
  WriterRequest? _pendingWriterRequest;
  String? _workspace;
  String? _clientId;
  String? _writerClientId;
  bool _requestingControl = false;
  bool _disposed = false;

  List<ChatMessage> get messages => List.unmodifiable(_messages);
  bool get loading => _loading;
  bool get loaded => _loaded;
  String? get loadError => _loadError;
  PendingAction? get pendingApproval => _pendingApproval;
  WriterRequest? get pendingWriterRequest => _pendingWriterRequest;
  String? get clientId => _clientId;
  String? get writerClientId => _writerClientId;
  bool get isWriter => _clientId != null && _clientId == _writerClientId;
  bool get requestingControl => _requestingControl;
  bool get canRequestControl =>
      _loaded &&
      _loadError == null &&
      !isWriter &&
      _pendingWriterRequest == null;
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
      _loaded &&
      _loadError == null &&
      isWriter &&
      !_loading &&
      _pendingApproval == null;

  Future<void> ensureLoaded() => _initFuture;

  Future<void> _init() async {
    try {
      final connected = await ApiService.connectSession(
        sessionId,
        clientId: _clientIdsBySession[sessionId],
      );
      _clientId = connected.clientId;
      _clientIdsBySession[sessionId] = connected.clientId;
      _applyAccess(connected.writerClientId, connected.pendingWriterRequest);
      _applySessionDetail(connected.session);
      _listenEvents();
    } catch (e) {
      _loadError = e.toString();
    } finally {
      _loaded = true;
      _notify();
    }
  }

  Future<void> send(String text) async {
    await _initFuture;
    final message = text.trim();
    if (message.isEmpty || !canSend) return;
    final clientId = _clientId;
    if (clientId == null) return;

    _messages.add(ChatMessage(role: 'user', text: message));
    _loading = true;
    _pendingApproval = null;
    _notify();

    try {
      await ApiService.stream(sessionId, message, clientId, _onSseEvent);
    } catch (e) {
      _add('assistant', 'Error: $e');
    } finally {
      _loading = false;
      _notify();
    }
  }

  Future<void> submitApproval(bool approved) async {
    await _initFuture;
    final action = _pendingApproval;
    final clientId = _clientId;
    if (action == null || clientId == null || !isWriter) return;

    _pendingApproval = null;
    _loading = true;
    _messages.add(
      ChatMessage(
        role: 'tool',
        text: approved ? 'Approval sent.' : 'Rejection sent.',
      ),
    );
    _notify();

    try {
      if (approved) {
        await ApiService.approve(sessionId, clientId);
      } else {
        await ApiService.reject(sessionId, clientId);
      }
    } catch (e) {
      _pendingApproval = action;
      _loading = false;
      _messages.add(ChatMessage(role: 'assistant', text: 'Approval error: $e'));
      _notify();
      rethrow;
    }
  }

  Future<void> requestControl() async {
    await _initFuture;
    final clientId = _clientId;
    if (clientId == null || !canRequestControl || _requestingControl) return;

    _requestingControl = true;
    _notify();
    try {
      final access = await ApiService.requestWriter(sessionId, clientId);
      _applyAccess(access.writerClientId, access.pendingWriterRequest);
      if (isWriter) {
        _messages.add(ChatMessage(role: 'assistant', text: 'Control granted.'));
      } else if (_pendingWriterRequest != null) {
        _messages.add(
          ChatMessage(
            role: 'assistant',
            text: 'Control request sent. Waiting for writer approval.',
          ),
        );
      }
    } catch (e) {
      _messages.add(ChatMessage(role: 'assistant', text: 'Control error: $e'));
    } finally {
      _requestingControl = false;
      _notify();
    }
  }

  Future<void> resolveControlRequest(bool approved) async {
    await _initFuture;
    final request = _pendingWriterRequest;
    final clientId = _clientId;
    if (request == null || clientId == null || !isWriter) return;

    try {
      final access = approved
          ? await ApiService.approveWriterRequest(
              sessionId,
              clientId,
              request.id,
            )
          : await ApiService.rejectWriterRequest(
              sessionId,
              clientId,
              request.id,
            );
      _applyAccess(access.writerClientId, access.pendingWriterRequest);
      _notify();
    } catch (e) {
      _messages.add(ChatMessage(role: 'assistant', text: 'Control error: $e'));
      _notify();
    }
  }

  void _listenEvents() {
    final clientId = _clientId;
    if (clientId == null || clientId.isEmpty) return;
    ApiService.events(sessionId, clientId, _onSseEvent).catchError((e) {
      _messages.add(
        ChatMessage(role: 'assistant', text: 'Live updates error: $e'),
      );
      _notify();
    });
  }

  void _onSseEvent(SseEvent event) {
    if (_disposed) return;
    switch (event) {
      case SseConnected(:final writerClientId, :final request):
        _applyAccess(writerClientId, request ?? _pendingWriterRequest);
        _notify();

      case SseMessage(:final clientId, :final role, :final content):
        if (!_isOwnEvent(clientId) && content.isNotEmpty) {
          _add(role, content);
        }

      case SseThinking():
        break;

      case SseToolStart(:final clientId, :final tool, :final args):
        if (!_isOwnEvent(clientId)) {
          _add('tool', '-> $tool${args != null ? ' $args' : ''}');
        }

      case SseToolDone(:final clientId, :final tool, :final result):
        if (!_isOwnEvent(clientId)) {
          final preview = (result != null && result.length > 120)
              ? '${result.substring(0, 120)}...'
              : (result ?? '');
          _add('tool', 'done $tool: $preview');
        }

      case SseToolError(:final clientId, :final tool, :final error):
        if (!_isOwnEvent(clientId)) {
          _add('tool', 'error $tool: $error');
        }

      case SseApprovalRequired(:final clientId, :final tool):
        if (!_isOwnEvent(clientId)) {
          final action = event.action ?? PendingAction(tool: tool);
          _pendingApproval = isWriter ? action : null;
          _loading = false;
          _messages.add(
            ChatMessage(
              role: 'assistant',
              text: action.summary ?? '"${action.tool}" requires approval',
            ),
          );
          _notify();
        }

      case SseDone(:final clientId, :final answer):
        _pendingApproval = null;
        if (!_isOwnEvent(clientId)) {
          _add('assistant', answer);
        }

      case SseClose():
        _pendingApproval = null;
        _notify();

      case SseWriterChanged(:final writerClientId):
        _applyAccess(writerClientId, null);
        _notify();

      case SseWriterRequestCreated(:final writerClientId, :final request):
        _applyAccess(writerClientId, request);
        _notify();

      case SseWriterRequestResolved(:final writerClientId):
        _applyAccess(writerClientId, null);
        _notify();

      case SseWorkspaceChanged():
        break;
    }
  }

  void _applySessionDetail(SessionDetailResponse detail) {
    _workspace = detail.workspace;
    _applyAccess(detail.writerClientId, detail.pendingWriterRequest);
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
    if (_disposed) return;
    _messages.add(ChatMessage(role: role, text: text));
    _notify();
  }

  void _applyAccess(String? writerClientId, WriterRequest? request) {
    _writerClientId = writerClientId;
    _pendingWriterRequest = request;
    if (!isWriter) {
      _pendingApproval = null;
      _loading = false;
    }
  }

  bool _isOwnEvent(String? eventClientId) {
    return eventClientId != null && eventClientId == _clientId;
  }

  void _notify() {
    if (!_disposed) {
      notifyListeners();
    }
  }

  @override
  void dispose() {
    _disposed = true;
    final clientId = _clientId;
    if (clientId != null && clientId.isNotEmpty && isWriter) {
      unawaited(_releaseWriterOnDispose(clientId));
    }
    super.dispose();
  }

  Future<void> _releaseWriterOnDispose(String clientId) async {
    try {
      await ApiService.releaseWriter(sessionId, clientId);
    } catch (_) {}
  }
}
