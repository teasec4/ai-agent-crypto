import 'package:flutter/material.dart';
import 'package:flutterapp/Session/service/api_service.dart';

class ApprovalBar extends StatelessWidget {
  final String sessionId;
  final String tool;
  final VoidCallback onReject;
  final VoidCallback onStartLoading;

  const ApprovalBar({
    super.key,
    required this.sessionId,
    required this.tool,
    required this.onReject,
    required this.onStartLoading,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('⚠️ "$tool" requires approval',
                style: Theme.of(context).textTheme.bodyMedium),
              const SizedBox(height: 8),
              Row(
                children: [
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () {
                        ApiService.reject(sessionId);
                        onReject();
                      },
                      icon: const Icon(Icons.close, size: 18),
                      label: const Text('Reject'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: Colors.red,
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: FilledButton.icon(
                      onPressed: () {
                        onStartLoading();
                        ApiService.approve(sessionId);
                      },
                      icon: const Icon(Icons.check, size: 18),
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
