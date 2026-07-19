import 'package:flutter/material.dart';
import 'package:flutterapp/Session/service/domain/response.dart';

class ProjectListTile extends StatelessWidget {
  final SessionSummary project;
  final VoidCallback onTap;

  const ProjectListTile({
    super.key,
    required this.project,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final title = projectTitle(project);
    final subtitle = project.workspace?.trim().isNotEmpty == true
        ? project.workspace!
        : '${project.messageCount} messages';

    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: const Icon(Icons.folder_outlined),
      title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis),
      trailing: Icon(Icons.chevron_right, color: theme.colorScheme.outline),
      onTap: onTap,
    );
  }
}

String projectTitle(SessionSummary project) {
  final workspace = project.workspace?.trim();
  if (workspace == null || workspace.isEmpty) {
    final shortId = project.id.length > 8
        ? project.id.substring(0, 8)
        : project.id;
    return 'Project $shortId';
  }
  final parts = workspace
      .replaceAll('\\', '/')
      .split('/')
      .where((part) => part.isNotEmpty)
      .toList();
  return parts.isEmpty ? workspace : parts.last;
}
