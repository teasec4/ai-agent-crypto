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
      appBar: AppBar(title: Text(chat.title)),
      body: Column(
        children: [
          Expanded(
            child: _ChatBody(chat: chat, scrollController: _scrollController),
          ),
          if (chat.loaded && chat.loadError == null) _ControlBar(chat: chat),
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

class _ControlBar extends StatelessWidget {
  final ChatController chat;

  const _ControlBar({required this.chat});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final request = chat.pendingWriterRequest;

    if (chat.isWriter && request != null) {
      return Padding(
        padding: const EdgeInsets.fromLTRB(12, 4, 12, 4),
        child: Row(
          children: [
            const Icon(Icons.sync_alt, size: 18),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'Control requested',
                style: theme.textTheme.bodyMedium,
              ),
            ),
            TextButton(
              onPressed: () => chat.resolveControlRequest(false),
              child: const Text('Reject'),
            ),
            FilledButton(
              onPressed: () => chat.resolveControlRequest(true),
              child: const Text('Approve'),
            ),
          ],
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 4, 12, 4),
      child: Row(
        children: [
          Icon(
            chat.isWriter ? Icons.edit : Icons.visibility_outlined,
            size: 18,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              chat.isWriter ? 'Writer' : 'View only',
              style: theme.textTheme.bodyMedium,
            ),
          ),
          if (!chat.isWriter)
            OutlinedButton(
              onPressed: chat.canRequestControl && !chat.requestingControl
                  ? chat.requestControl
                  : null,
              child: Text(request == null ? 'Request control' : 'Requested'),
            ),
        ],
      ),
    );
  }
}

class _ChatBody extends StatelessWidget {
  final ChatController chat;
  final ScrollController scrollController;

  const _ChatBody({required this.chat, required this.scrollController});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    if (!chat.loaded) {
      return const Center(child: CircularProgressIndicator());
    }

    if (chat.loadError != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.error_outline,
                size: 44,
                color: theme.colorScheme.error,
              ),
              const SizedBox(height: 12),
              Text(
                'Could not open project',
                style: theme.textTheme.titleMedium,
              ),
              const SizedBox(height: 6),
              Text(chat.loadError!, textAlign: TextAlign.center),
            ],
          ),
        ),
      );
    }

    if (chat.messages.isEmpty) {
      return Center(
        child: Text(
          'Start the conversation.',
          style: theme.textTheme.bodyMedium,
        ),
      );
    }

    return ListView.builder(
      controller: scrollController,
      padding: const EdgeInsets.all(12),
      itemCount: chat.messages.length,
      itemBuilder: (_, i) => MessageBubble(message: chat.messages[i]),
    );
  }
}
