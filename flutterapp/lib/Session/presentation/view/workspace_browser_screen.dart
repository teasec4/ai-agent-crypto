import 'package:flutter/material.dart';
import 'package:flutterapp/Session/presentation/view/detail_screen.dart';
import 'package:flutterapp/Session/service/api_service.dart';
import 'package:flutterapp/Session/service/domain/response.dart';

class WorkspaceBrowserScreen extends StatefulWidget {
  const WorkspaceBrowserScreen({super.key});

  @override
  State<WorkspaceBrowserScreen> createState() => _WorkspaceBrowserScreenState();
}

class _WorkspaceBrowserScreenState extends State<WorkspaceBrowserScreen> {
  List<WorkspaceRoot> _roots = const [];
  WorkspaceBrowseResponse? _listing;
  bool _loading = true;
  bool _creating = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadInitial();
  }

  Future<void> _loadInitial() async {
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final roots = await ApiService.listWorkspaceRoots();
      final initialPath = roots.isNotEmpty ? roots.first.path : '';
      final listing = await ApiService.browseWorkspace(initialPath);
      if (!mounted) return;
      setState(() {
        _roots = listing.roots.isNotEmpty ? listing.roots : roots;
        _listing = listing;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _browse(String path) async {
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final listing = await ApiService.browseWorkspace(path);
      if (!mounted) return;
      setState(() {
        _roots = listing.roots.isNotEmpty ? listing.roots : _roots;
        _listing = listing;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _createProject() async {
    final path = _listing?.path;
    if (path == null || path.isEmpty || _creating) return;

    setState(() {
      _creating = true;
      _error = null;
    });

    try {
      final sessionId = await ApiService.createSession();
      final connected = await ApiService.connectSession(sessionId);
      await ApiService.setWorkspace(
        sessionId,
        path,
        clientId: connected.clientId,
      );
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => DetailScreen(sessionId: sessionId)),
      );
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _creating = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final listing = _listing;

    return Scaffold(
      appBar: AppBar(title: const Text('New Project')),
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (_roots.isNotEmpty)
                    SingleChildScrollView(
                      scrollDirection: Axis.horizontal,
                      child: Row(
                        children: [
                          for (final root in _roots)
                            Padding(
                              padding: const EdgeInsets.only(right: 8),
                              child: ActionChip(
                                avatar: Icon(_rootIcon(root.kind), size: 18),
                                label: Text(root.name),
                                onPressed: _loading
                                    ? null
                                    : () => _browse(root.path),
                              ),
                            ),
                        ],
                      ),
                    ),
                  Text(
                    'Server: ${ApiService.baseUri}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.outline,
                    ),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: SelectableText(
                          listing?.path ?? '',
                          maxLines: 2,
                          style: theme.textTheme.bodySmall,
                        ),
                      ),
                      IconButton(
                        onPressed: listing?.parentPath == null || _loading
                            ? null
                            : () => _browse(listing!.parentPath!),
                        icon: const Icon(Icons.arrow_upward),
                        tooltip: 'Up',
                      ),
                    ],
                  ),
                  if (_error != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(
                        _error!,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.error,
                        ),
                      ),
                    ),
                ],
              ),
            ),
            const Divider(height: 1),
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null && listing == null
                  ? _WorkspaceErrorState(error: _error!, onRetry: _loadInitial)
                  : _DirectoryList(
                      entries: listing?.entries ?? const [],
                      onOpen: _browse,
                    ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 10, 16, 16),
              child: FilledButton.icon(
                onPressed: listing == null || _loading || _creating
                    ? null
                    : _createProject,
                icon: _creating
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.check),
                label: const Text('Create Project Here'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  IconData _rootIcon(String kind) {
    switch (kind) {
      case 'home':
        return Icons.home_outlined;
      case 'recent':
        return Icons.history;
      default:
        return Icons.computer_outlined;
    }
  }
}

class _WorkspaceErrorState extends StatelessWidget {
  final String error;
  final VoidCallback onRetry;

  const _WorkspaceErrorState({required this.error, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.cloud_off_outlined,
              size: 44,
              color: theme.colorScheme.error,
            ),
            const SizedBox(height: 12),
            Text('Could not load folders', style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(error, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            OutlinedButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh),
              label: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }
}

class _DirectoryList extends StatelessWidget {
  final List<WorkspaceEntry> entries;
  final ValueChanged<String> onOpen;

  const _DirectoryList({required this.entries, required this.onOpen});

  @override
  Widget build(BuildContext context) {
    final folders = entries.where((entry) => entry.isDir).toList();

    if (folders.isEmpty) {
      return const Center(child: Text('No folders here.'));
    }

    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      itemCount: folders.length,
      separatorBuilder: (_, _) => const Divider(height: 1),
      itemBuilder: (context, index) {
        final entry = folders[index];
        return ListTile(
          contentPadding: EdgeInsets.zero,
          leading: const Icon(Icons.folder_outlined),
          title: Text(entry.name, maxLines: 1, overflow: TextOverflow.ellipsis),
          subtitle: Text(
            entry.path,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => onOpen(entry.path),
        );
      },
    );
  }
}
