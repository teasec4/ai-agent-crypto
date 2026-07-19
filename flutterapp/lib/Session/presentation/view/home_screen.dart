import 'package:flutter/material.dart';
import 'package:flutterapp/Session/presentation/view/detail_screen.dart';
import 'package:flutterapp/Session/presentation/view/workspace_browser_screen.dart';
import 'package:flutterapp/Session/presentation/view/widgets/project_list_tile.dart';
import 'package:flutterapp/Session/service/api_service.dart';
import 'package:flutterapp/Session/service/domain/response.dart';

const _lanServerPreset = 'http://192.168.31.89:8080';

class ProjectHubScreen extends StatefulWidget {
  const ProjectHubScreen({super.key});

  @override
  State<ProjectHubScreen> createState() => _ProjectHubScreenState();
}

class _ProjectHubScreenState extends State<ProjectHubScreen> {
  late Future<List<SessionSummary>> _projectsFuture;
  late final TextEditingController _serverController;
  String? _serverStatus;

  @override
  void initState() {
    super.initState();
    _serverController = TextEditingController(
      text: ApiService.baseUri.toString(),
    );
    _projectsFuture = ApiService.listSessions();
  }

  @override
  void dispose() {
    _serverController.dispose();
    super.dispose();
  }

  void _reload() {
    setState(() {
      _projectsFuture = ApiService.listSessions();
    });
  }

  void _applyServerUrl() {
    try {
      final uri = ApiService.setBaseUrl(_serverController.text);
      _serverController.text = uri.toString();
      setState(() {
        _serverStatus = 'Using $uri';
        _projectsFuture = ApiService.listSessions();
      });
    } catch (e) {
      setState(() {
        _serverStatus = e.toString();
      });
    }
  }

  void _useLanServerPreset() {
    _serverController.text = _lanServerPreset;
    _applyServerUrl();
  }

  void _openNewProject() {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (_) => const WorkspaceBrowserScreen()));
  }

  void _openProject(SessionSummary project) {
    Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => DetailScreen(sessionId: project.id)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('AI Agent')),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(20),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 560),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const Icon(Icons.terminal_rounded, size: 64),
                  const SizedBox(height: 16),
                  Text(
                    'Choose a project',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.headlineSmall,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Start in a folder on the agent computer, or reopen recent work.',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodyMedium,
                  ),
                  const SizedBox(height: 24),
                  FilledButton.icon(
                    onPressed: _openNewProject,
                    icon: const Icon(Icons.add),
                    label: const Text('New Project'),
                  ),
                  const SizedBox(height: 16),
                  ExpansionTile(
                    tilePadding: EdgeInsets.zero,
                    title: const Text('Server'),
                    subtitle: Text(
                      ApiService.baseUri.toString(),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    children: [
                      TextField(
                        controller: _serverController,
                        decoration: const InputDecoration(
                          labelText: 'API URL',
                          hintText: _lanServerPreset,
                          border: OutlineInputBorder(),
                        ),
                        keyboardType: TextInputType.url,
                        textInputAction: TextInputAction.done,
                        onSubmitted: (_) => _applyServerUrl(),
                      ),
                      const SizedBox(height: 8),
                      Wrap(
                        alignment: WrapAlignment.end,
                        spacing: 8,
                        runSpacing: 8,
                        children: [
                          ActionChip(
                            avatar: const Icon(Icons.wifi, size: 18),
                            label: const Text('192.168.31.89'),
                            onPressed: _useLanServerPreset,
                          ),
                          OutlinedButton.icon(
                            onPressed: _applyServerUrl,
                            icon: const Icon(Icons.link),
                            label: const Text('Use Server'),
                          ),
                        ],
                      ),
                      if (_serverStatus != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Align(
                            alignment: Alignment.centerLeft,
                            child: Text(
                              _serverStatus!,
                              style: theme.textTheme.bodySmall,
                            ),
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(height: 20),
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          'Recent projects',
                          style: theme.textTheme.titleMedium,
                        ),
                      ),
                      IconButton(
                        onPressed: _reload,
                        icon: const Icon(Icons.refresh),
                        tooltip: 'Refresh',
                      ),
                    ],
                  ),
                  FutureBuilder<List<SessionSummary>>(
                    future: _projectsFuture,
                    builder: (context, snapshot) {
                      if (snapshot.connectionState == ConnectionState.waiting) {
                        return const Padding(
                          padding: EdgeInsets.symmetric(vertical: 28),
                          child: Center(child: CircularProgressIndicator()),
                        );
                      }
                      if (snapshot.hasError) {
                        return _ErrorBlock(
                          message:
                              'Could not reach the agent API\n${snapshot.error}',
                          onRetry: _reload,
                        );
                      }

                      final projects = snapshot.data ?? const [];
                      if (projects.isEmpty) {
                        return Padding(
                          padding: const EdgeInsets.symmetric(vertical: 20),
                          child: Text(
                            'No projects yet.',
                            textAlign: TextAlign.center,
                            style: theme.textTheme.bodyMedium,
                          ),
                        );
                      }

                      return Column(
                        children: [
                          for (final project in projects)
                            ProjectListTile(
                              project: project,
                              onTap: () => _openProject(project),
                            ),
                        ],
                      );
                    },
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ErrorBlock extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;

  const _ErrorBlock({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 20),
      child: Column(
        children: [
          Icon(
            Icons.cloud_off_outlined,
            size: 42,
            color: theme.colorScheme.error,
          ),
          const SizedBox(height: 12),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            onPressed: onRetry,
            icon: const Icon(Icons.refresh),
            label: const Text('Retry'),
          ),
        ],
      ),
    );
  }
}
