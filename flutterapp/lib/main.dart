import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';

void main() => runApp(const MyApp());

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'AI Agent',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorSchemeSeed: Colors.blueAccent,
        useMaterial3: true,
      ),
      home: const ChatScreen(),
    );
  }
}

class ChatScreen extends StatefulWidget {
  const ChatScreen({super.key});

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  static const _host = 'http://localhost:8080';

  final _controller = TextEditingController();
  final _messages = <ChatMessage>[];
  String _sessionId = '';
  bool _loading = false;

  @override
  void initState() {
    super.initState();
    _createSession();
  }

  Future<void> _createSession() async {
    final client = HttpClient();
    try {
      final req = await client.postUrl(Uri.parse('$_host/sessions'));
      final res = await req.close();
      final body = await res.transform(utf8.decoder).join();
      _sessionId = jsonDecode(body)['sessionId'] as String;
    } finally {
      client.close();
    }
  }

  Future<void> _send(String text) async {
    if (text.isEmpty) return;
    _controller.clear();

    setState(() {
      _messages.add(ChatMessage(role: 'user', content: text));
      _loading = true;
    });

    try {
      await _streamTask(text);
    } catch (e) {
      setState(() => _messages.add(ChatMessage(role: 'assistant', content: 'Error: $e')));
    } finally {
      setState(() => _loading = false);
    }
  }

  Future<void> _streamTask(String message) async {
    final client = HttpClient();
    try {
      final req = await client.postUrl(
        Uri.parse('$_host/sessions/$_sessionId/stream'),
      );
      req.headers.contentType = ContentType.json;
      req.write(jsonEncode({'message': message}));

      final res = await req.close();
      final lines = res.transform(utf8.decoder).transform(const LineSplitter());

      final buffer = StringBuffer();
      await for (final line in lines) {
        if (line.startsWith('data: ')) {
          final data = line.substring(6);
          final event = jsonDecode(data) as Map<String, dynamic>;
          final type = event['type'] as String?;

          switch (type) {
            case 'tool_start':
              final tool = event['tool'];
              final args = event['args'] as Map?;
              final preview = '→ $tool${args != null ? ' ${jsonEncode(args)}' : ''}';
              setState(() => _messages.add(ChatMessage(role: 'tool', content: preview)));
              break;

            case 'tool_done':
              final tool = event['tool'];
              final result = event['result'] as String? ?? '';
              final preview = '✓ $tool: ${result.length > 120 ? '${result.substring(0, 120)}...' : result}';
              setState(() => _messages.add(ChatMessage(role: 'tool', content: preview)));
              break;

            case 'done':
              final answer = event['answer'] as String? ?? '';
              setState(() => _messages.add(ChatMessage(role: 'assistant', content: answer)));
              break;

            case 'approval_required':
              setState(() => _messages.add(ChatMessage(
                role: 'assistant',
                content: '⚠️ Action "${event['tool']}" needs approval. POST /sessions/$_sessionId/approve to confirm.',
              )));
              break;

            case 'thinking':
              // показываем точки, но не спамим
              break;
          }
        }
      }
    } finally {
      client.close();
    }
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
          // Message list
          Expanded(
            child: ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: _messages.length,
              itemBuilder: (context, i) {
                final msg = _messages[i];
                final isUser = msg.role == 'user';
                final isTool = msg.role == 'tool';
                return Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: isTool
                          ? Colors.amber.withOpacity(0.1)
                          : isUser
                              ? Colors.blue.withOpacity(0.1)
                              : Colors.grey.withOpacity(0.1),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Text(
                      msg.content,
                      style: TextStyle(
                        fontFamily: isTool ? 'monospace' : null,
                        fontSize: 14,
                      ),
                    ),
                  ),
                );
              },
            ),
          ),

          // Loading indicator
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
            padding: const EdgeInsets.all(8),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _controller,
                    decoration: const InputDecoration(
                      hintText: 'Ask the agent...',
                      border: OutlineInputBorder(),
                      contentPadding: EdgeInsets.symmetric(horizontal: 12),
                    ),
                    onSubmitted: _send,
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

class ChatMessage {
  final String role;
  final String content;
  ChatMessage({required this.role, required this.content});
}
