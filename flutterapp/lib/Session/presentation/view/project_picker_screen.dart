import 'package:flutter/material.dart';
import 'package:flutterapp/Session/presentation/view/detail_screen.dart';
import 'package:flutterapp/Session/presentation/view/workspace_browser_screen.dart';
import 'package:flutterapp/Session/presentation/view/widgets/project_list_tile.dart';
import 'package:flutterapp/Session/service/api_service.dart';
import 'package:flutterapp/Session/service/domain/response.dart';

class ProjectPickerScreen extends StatefulWidget {
  const ProjectPickerScreen({super.key});

  @override
  State<ProjectPickerScreen> createState() => _ProjectPickerScreenState();
}

class _ProjectPickerScreenState extends State<ProjectPickerScreen> {
  late Future<List<SessionSummary>> _projectsFuture;

  @override
  void initState() {
    super.initState();
    _projectsFuture = ApiService.listSessions();
  }

  void _reload() {
    setState(() {
      _projectsFuture = ApiService.listSessions();
    });
  }

  void _openProject(SessionSummary project) {
    Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => DetailScreen(sessionId: project.id)),
    );
  }

  void _createProject() {
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => const WorkspaceBrowserScreen()),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Open Existing'),
        actions: [
          IconButton(
            onPressed: _reload,
            icon: const Icon(Icons.refresh),
            tooltip: 'Refresh',
          ),
        ],
      ),
      body: FutureBuilder<List<SessionSummary>>(
        future: _projectsFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return _CenteredState(
              icon: Icons.cloud_off_outlined,
              title: 'Could not load projects',
              body: snapshot.error.toString(),
              action: OutlinedButton.icon(
                onPressed: _reload,
                icon: const Icon(Icons.refresh),
                label: const Text('Retry'),
              ),
            );
          }

          final projects = snapshot.data ?? const [];
          if (projects.isEmpty) {
            return _CenteredState(
              icon: Icons.folder_off_outlined,
              title: 'No projects yet',
              body: 'Create one from a folder on the agent computer.',
              action: FilledButton.icon(
                onPressed: _createProject,
                icon: const Icon(Icons.add),
                label: const Text('New Project'),
              ),
            );
          }

          return ListView.separated(
            padding: const EdgeInsets.all(20),
            itemCount: projects.length,
            separatorBuilder: (_, _) => Divider(color: theme.dividerColor),
            itemBuilder: (context, index) {
              final project = projects[index];
              return ProjectListTile(
                project: project,
                onTap: () => _openProject(project),
              );
            },
          );
        },
      ),
    );
  }
}

class _CenteredState extends StatelessWidget {
  final IconData icon;
  final String title;
  final String body;
  final Widget action;

  const _CenteredState({
    required this.icon,
    required this.title,
    required this.body,
    required this.action,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 48, color: theme.colorScheme.outline),
            const SizedBox(height: 14),
            Text(title, style: theme.textTheme.titleMedium),
            const SizedBox(height: 6),
            Text(body, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            action,
          ],
        ),
      ),
    );
  }
}
