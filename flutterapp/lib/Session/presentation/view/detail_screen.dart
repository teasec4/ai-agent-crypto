import 'package:flutter/material.dart';
import 'package:flutterapp/Session/presentation/view/widgets/approval_bar.dart';
import 'package:flutterapp/Session/presentation/view/widgets/chat_input.dart';
import 'package:flutterapp/Session/presentation/view/widgets/message_bubble.dart';
import 'package:flutterapp/Session/service/api_service.dart';
import 'package:flutterapp/Session/service/domain/chat_message.dart';

class DetailScreen extends StatefulWidget {
  final String sessionId;
  const DetailScreen({super.key, required this.sessionId});

  @override
  State<DetailScreen> createState() => _DetailScreenState();
}

class _DetailScreenState extends State<DetailScreen> {
  final _controller = TextEditingController();
  final _messages = <ChatMessage>[];
  bool _loading = false;
  String? _pendingApprovalTool;

  Future<void> _send(String text) async {
    if (text.isEmpty || text == _controller.text) return;
    _controller.clear();

    setState(() {
      _messages.add(ChatMessage(role: 'user', text: text));
      _loading = true;
      _pendingApprovalTool = null;
    });

    try {
      await ApiService.stream(
        widget.sessionId,
        text,
        _onSseEvent,
      );
    } catch (e) {
      _add('assistant', 'Error: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _onSseEvent(SseEvent event) {
    switch (event) {
      case SseThinking():
        break;

      case SseToolStart(:final tool, :final args):
        _add('tool', '→ $tool${args != null ? ' $args' : ''}');

      case SseToolDone(:final tool, :final result):
        final preview = (result != null && result.length > 120)
            ? '${result.substring(0, 120)}...'
            : (result ?? '');
        _add('tool', '✓ $tool: $preview');

      case SseToolError(:final tool, :final error):
        _add('tool', '✗ $tool: $error');

      case SseApprovalRequired(:final tool):
        _pendingApprovalTool = tool;
        _add('assistant', '⚠️ "$tool" requires approval');
        if (mounted) setState(() => _loading = false);

      case SseDone(:final answer):
        _add('assistant', answer);

      case SseClose():
        break;
    }
  }

  void _add(String role, String text) {
    if (!mounted) return;
    setState(() => _messages.add(ChatMessage(role: role, text: text)));
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('AI Agent')),
      body: Column(
        children: [
          // Messages
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: _messages.length,
              itemBuilder: (_, i) => MessageBubble(message: _messages[i]),
            ),
          ),

          // Approval bar
          if (_pendingApprovalTool != null)
            ApprovalBar(
              sessionId: widget.sessionId,
              tool: _pendingApprovalTool!,
              onReject: () => _pendingApprovalTool = null,
              onStartLoading: () => setState(() => _loading = true),
            ),

          // Spinner
          if (_loading)
            const Padding(
              padding: EdgeInsets.only(bottom: 4),
              child: SizedBox(
                width: 16, height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),

          // Input
          ChatInput(
            controller: _controller,
            enabled: !_loading,
            onSubmitted: _send,
          ),
        ],
      ),
    );
  }
}
