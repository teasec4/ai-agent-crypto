import 'package:flutter/material.dart';
import 'package:flutterapp/Session/service/api_service.dart';

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
    if (text.isEmpty) return;
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
      _addMessage('assistant', 'Error: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _onSseEvent(SseEvent event) {
    switch (event) {
      case SseThinking():
        break; // тихо, спиннер уже есть

      case SseToolStart(:final tool, :final args):
        final preview = args != null ? ' ${args.toString()}' : '';
        _addMessage('tool', '→ $tool$preview');

      case SseToolDone(:final tool, :final result):
        final preview = result != null && result.length > 120
            ? '${result.substring(0, 120)}...'
            : (result ?? '');
        _addMessage('tool', '✓ $tool: $preview');

      case SseToolError(:final tool, :final error):
        _addMessage('tool', '✗ $tool: $error');

      case SseApprovalRequired(:final tool):
        _pendingApprovalTool = tool;
        _addMessage('assistant', '⚠️ $tool требует подтверждения');
        if (mounted) setState(() => _loading = false);

      case SseDone(:final answer):
        _addMessage('assistant', answer);

      case SseClose():
        break;
    }
  }

  void _addMessage(String role, String text) {
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
              itemBuilder: (_, i) => _MessageBubble(msg: _messages[i]),
            ),
          ),

          // Approval buttons
          if (_pendingApprovalTool != null)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
              child: Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () async {
                        await ApiService.reject(widget.sessionId);
                        _pendingApprovalTool = null;
                      },
                      icon: const Icon(Icons.close),
                      label: const Text('Reject'),
                      style: OutlinedButton.styleFrom(foregroundColor: Colors.red),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: FilledButton.icon(
                      onPressed: () async {
                        setState(() => _loading = true);
                        await ApiService.approve(widget.sessionId);
                        _pendingApprovalTool = null;
                      },
                      icon: const Icon(Icons.check),
                      label: const Text('Approve'),
                    ),
                  ),
                ],
              ),
            ),

          // Loading
          if (_loading)
            const Padding(
              padding: EdgeInsets.only(bottom: 4),
              child: SizedBox(
                width: 16, height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),

          // Input
          Padding(
            padding: const EdgeInsets.fromLTRB(8, 4, 8, 8),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _controller,
                    decoration: const InputDecoration(
                      hintText: 'Ask the agent...',
                      border: OutlineInputBorder(),
                      contentPadding: EdgeInsets.symmetric(horizontal: 12),
                      isDense: true,
                    ),
                    onSubmitted: _loading ? null : _send,
                  ),
                ),
                const SizedBox(width: 8),
                IconButton(
                  onPressed: _loading ? null : () => _send(_controller.text),
                  icon: const Icon(Icons.send),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ---- Chat Bubble Widget ----

class _MessageBubble extends StatelessWidget {
  final ChatMessage msg;
  const _MessageBubble({required this.msg});

  @override
  Widget build(BuildContext context) {
    final isUser = msg.role == 'user';
    final isTool = msg.role == 'tool';
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: isTool
              ? Colors.amber.withOpacity(0.08)
              : isUser
                  ? Colors.blue.withOpacity(0.08)
                  : Colors.grey.withOpacity(0.08),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Text(
          msg.text,
          style: TextStyle(
            fontFamily: isTool ? 'monospace' : null,
            fontSize: 14,
          ),
        ),
      ),
    );
  }
}

// ---- Data Model ----

class ChatMessage {
  final String role;
  final String text;
  ChatMessage({required this.role, required this.text});
}
