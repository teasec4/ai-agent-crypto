import 'package:flutter/material.dart';
import 'package:flutterapp/Session/service/domain/sse_event.dart';

class ApprovalBar extends StatefulWidget {
  final PendingAction action;
  final Future<void> Function(bool approved) onDecision;

  const ApprovalBar({
    super.key,
    required this.action,
    required this.onDecision,
  });

  @override
  State<ApprovalBar> createState() => _ApprovalBarState();
}

class _ApprovalBarState extends State<ApprovalBar> {
  bool _submitting = false;
  String? _error;

  Future<void> _decide(bool approved) async {
    setState(() {
      _submitting = true;
      _error = null;
    });

    try {
      await widget.onDecision(approved);
      if (!mounted) return;
      setState(() => _submitting = false);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _error = e.toString();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final action = widget.action;
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                children: [
                  const Icon(Icons.verified_user_outlined, size: 20),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      action.summary ?? '${action.tool} requires approval',
                      style: theme.textTheme.titleSmall,
                    ),
                  ),
                  if (action.risk != null)
                    Chip(
                      label: Text(action.risk!),
                      visualDensity: VisualDensity.compact,
                    ),
                ],
              ),
              if (action.preview != null && action.preview!.trim().isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Container(
                    constraints: const BoxConstraints(maxHeight: 180),
                    padding: const EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: theme.colorScheme.surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: SingleChildScrollView(
                      child: Text(
                        action.preview!,
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontFamily: 'monospace',
                        ),
                      ),
                    ),
                  ),
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
              const SizedBox(height: 10),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: _submitting ? null : () => _decide(false),
                      icon: const Icon(Icons.close, size: 18),
                      label: const Text('Reject'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: theme.colorScheme.error,
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: FilledButton.icon(
                      onPressed: _submitting ? null : () => _decide(true),
                      icon: _submitting
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Icon(Icons.check, size: 18),
                      label: const Text('Approve'),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
