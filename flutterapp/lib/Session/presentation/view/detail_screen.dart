import 'package:flutter/material.dart';
import 'package:flutterapp/Session/presentation/controller/chat_controller.dart';
import 'package:flutterapp/Session/presentation/view/widgets/approval_bar.dart';
import 'package:flutterapp/Session/presentation/view/widgets/chat_input.dart';
import 'package:flutterapp/Session/presentation/view/widgets/message_bubble.dart';
import 'package:provider/provider.dart';

class DetailScreen extends StatelessWidget {
  final String sessionId;
  const DetailScreen({super.key, required this.sessionId});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider(
      create: (_) => ChatController(sessionId: sessionId),
      child: const _DetailScreenBody(),
    );
  }
}

class _DetailScreenBody extends StatefulWidget {
  const _DetailScreenBody();

  @override
  State<_DetailScreenBody> createState() => _DetailScreenBodyState();
}

class _DetailScreenBodyState extends State<_DetailScreenBody> {
  final _inputController = TextEditingController();
  final _scrollController = ScrollController();
  int _lastMessageCount = 0;

  Future<void> _send(String text) async {
    _inputController.clear();
    await context.read<ChatController>().send(text);
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 180),
        curve: Curves.easeOut,
      );
    });
  }

  @override
  void dispose() {
    _inputController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final chat = context.watch<ChatController>();
    final pendingApproval = chat.pendingApproval;

    if (chat.messages.length != _lastMessageCount) {
      _lastMessageCount = chat.messages.length;
      _scrollToBottom();
    }

    return Scaffold(
      appBar: AppBar(title: const Text('AI Agent')),
      body: Column(
        children: [
          Expanded(
            child: ListView.builder(
              controller: _scrollController,
              padding: const EdgeInsets.all(12),
              itemCount: chat.messages.length,
              itemBuilder: (_, i) => MessageBubble(message: chat.messages[i]),
            ),
          ),
          if (pendingApproval != null)
            ApprovalBar(
              action: pendingApproval,
              onDecision: context.read<ChatController>().submitApproval,
            ),
          if (chat.loading)
            const Padding(
              padding: EdgeInsets.only(bottom: 4),
              child: SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),
          ChatInput(
            controller: _inputController,
            enabled: chat.canSend,
            onSubmitted: _send,
          ),
        ],
      ),
    );
  }
}
