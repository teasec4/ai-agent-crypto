import 'package:flutter/material.dart';
import 'package:flutterapp/Session/presentation/view/detail_screen.dart';
import 'package:flutterapp/Session/service/api_service.dart';

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
      home: const RootView(),
    );
  }
}

class RootView extends StatefulWidget {
  const RootView({super.key});

  @override
  State<RootView> createState() => _RootViewState();
}

class _RootViewState extends State<RootView> {
  late Future<String> _sessionFuture;

  @override
  void initState() {
    super.initState();
    _sessionFuture = ApiService.createSession();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('AI Agent')),
      body: FutureBuilder<String>(
        future: _sessionFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                CircularProgressIndicator(),
                SizedBox(height: 16),
                Text('Creating session...'),
              ],
            ));
          }

          if (snapshot.hasError) {
            return Center(child: Text(
              'Failed to connect to server\n${snapshot.error}',
              textAlign: TextAlign.center,
            ));
          }

          final sessionId = snapshot.data!;
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.smart_toy_outlined, size: 64, color: Colors.blueAccent),
                const SizedBox(height: 16),
                Text('Session: ${sessionId.substring(0, 16)}...',
                  style: Theme.of(context).textTheme.bodySmall),
                const SizedBox(height: 24),
                FilledButton.icon(
                  onPressed: () => Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => DetailScreen(sessionId: sessionId),
                    ),
                  ),
                  icon: const Icon(Icons.chat),
                  label: const Text('Start Chat'),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
