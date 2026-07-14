import 'package:flutter/material.dart';
import 'package:flutterapp/Session/service/domain/chat_message.dart';

class MessageBubble extends StatelessWidget {
  final ChatMessage message;

  const MessageBubble({super.key, required this.message});

  @override
  Widget build(BuildContext context) {
    final isUser = message.role == 'user';
    final isTool = message.role == 'tool';
    final color = isTool
        ? Colors.amber.withOpacity(0.08)
        : isUser
            ? Colors.blue.withOpacity(0.08)
            : Colors.grey.withOpacity(0.08);

    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: color,
          borderRadius: BorderRadius.circular(12),
        ),
        child: Text(
          message.text,
          style: TextStyle(
            fontFamily: isTool ? 'monospace' : null,
            fontSize: 14,
          ),
        ),
      ),
    );
  }
}
