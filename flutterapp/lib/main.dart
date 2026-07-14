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
      theme: ThemeData(colorSchemeSeed: Colors.blueAccent, useMaterial3: true),
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
  final _workspaceController = TextEditingController();
  bool _settingWorkspace = false;
  String? _workspaceStatus;

  @override
  void initState() {
    super.initState();
    _sessionFuture = ApiService.getOrCreateSession();
  }

  @override
  void dispose() {
    _workspaceController.dispose();
    super.dispose();
  }

  void _retrySession() {
    setState(() {
      _workspaceStatus = null;
      _sessionFuture = ApiService.getOrCreateSession();
    });
  }

  Future<void> _startFreshSession() async {
    setState(() {
      _workspaceStatus = 'Starting a fresh session...';
    });

    try {
      final sessionId = await ApiService.createSession();
      if (!mounted) return;
      setState(() {
        _sessionFuture = Future.value(sessionId);
        _workspaceStatus = null;
      });
      await Navigator.of(context).push(
        MaterialPageRoute(builder: (_) => DetailScreen(sessionId: sessionId)),
      );
    } catch (e) {
      if (!mounted) return;
      setState(() => _workspaceStatus = 'Could not start session: $e');
    }
  }

  Future<void> _setWorkspace(String sessionId) async {
    final path = _workspaceController.text.trim();
    if (path.isEmpty) {
      setState(
        () => _workspaceStatus =
            'Workspace is optional. Chat will use the API server launch directory.',
      );
      return;
    }

    setState(() {
      _settingWorkspace = true;
      _workspaceStatus = null;
    });

    try {
      await ApiService.setWorkspace(sessionId, path);
      if (!mounted) return;
      setState(() => _workspaceStatus = 'Workspace set: $path');
    } catch (e) {
      if (!mounted) return;
      setState(() => _workspaceStatus = 'Workspace error: $e');
    } finally {
      if (mounted) setState(() => _settingWorkspace = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('AI Agent')),
      body: FutureBuilder<String>(
        future: _sessionFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  CircularProgressIndicator(),
                  SizedBox(height: 16),
                  Text('Connecting to agent...'),
                ],
              ),
            );
          }

          if (snapshot.hasError) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.cloud_off_outlined, size: 48),
                    const SizedBox(height: 16),
                    Text(
                      'Could not reach the agent API\n${snapshot.error}',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 16),
                    FilledButton.icon(
                      onPressed: _retrySession,
                      icon: const Icon(Icons.refresh),
                      label: const Text('Retry'),
                    ),
                  ],
                ),
              ),
            );
          }

          final sessionId = snapshot.data!;
          final shortSessionId = sessionId.length > 16
              ? '${sessionId.substring(0, 16)}...'
              : sessionId;

          return Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 520),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Icon(Icons.smart_toy_outlined, size: 64),
                    const SizedBox(height: 16),
                    Text(
                      'Ready to Chat',
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.headlineSmall,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Continue the latest backend session, or start fresh if you want a clean conversation.',
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                    const SizedBox(height: 20),
                    FilledButton.icon(
                      onPressed: () => Navigator.of(context).push(
                        MaterialPageRoute(
                          builder: (_) => DetailScreen(sessionId: sessionId),
                        ),
                      ),
                      icon: const Icon(Icons.chat),
                      label: const Text('Open Chat'),
                    ),
                    const SizedBox(height: 8),
                    OutlinedButton.icon(
                      onPressed: _startFreshSession,
                      icon: const Icon(Icons.add_comment_outlined),
                      label: const Text('Start Fresh Session'),
                    ),
                    if (_workspaceStatus != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: Text(
                          _workspaceStatus!,
                          textAlign: TextAlign.center,
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                      ),
                    const SizedBox(height: 12),
                    ExpansionTile(
                      tilePadding: EdgeInsets.zero,
                      title: const Text('Project Folder (Optional)'),
                      subtitle: const Text(
                        'By default the agent uses the API launch folder.',
                      ),
                      children: [
                        TextField(
                          controller: _workspaceController,
                          decoration: const InputDecoration(
                            labelText: 'Path on API server',
                            hintText: '/Users/me/project',
                            border: OutlineInputBorder(),
                            helperText:
                                'Use this only when the API should work in another project folder.',
                          ),
                          onSubmitted: (_) => _setWorkspace(sessionId),
                        ),
                        const SizedBox(height: 8),
                        Align(
                          alignment: Alignment.centerRight,
                          child: OutlinedButton.icon(
                            onPressed: _settingWorkspace
                                ? null
                                : () => _setWorkspace(sessionId),
                            icon: _settingWorkspace
                                ? const SizedBox(
                                    width: 16,
                                    height: 16,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : const Icon(Icons.folder_open),
                            label: const Text('Apply Workspace'),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    ExpansionTile(
                      tilePadding: EdgeInsets.zero,
                      title: const Text('Connection Details'),
                      children: [
                        Align(
                          alignment: Alignment.centerLeft,
                          child: SelectableText('Session: $shortSessionId'),
                        ),
                        const SizedBox(height: 4),
                        Align(
                          alignment: Alignment.centerLeft,
                          child: SelectableText('API: ${ApiService.baseUri}'),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
