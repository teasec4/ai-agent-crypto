import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutterapp/Session/presentation/view/widgets/approval_bar.dart';
import 'package:flutterapp/Session/presentation/view/detail_screen.dart';
import 'package:flutterapp/Session/service/api_service.dart';
import 'package:flutterapp/Session/service/domain/sse_event.dart';
import 'package:flutterapp/main.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

void main() {
  setUp(() {
    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
    );
  });

  testWidgets('reuses an existing session before entering chat', (
    tester,
  ) async {
    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
      client: MockClient((request) async {
        expect(request.method, 'GET');
        expect(request.url.path, '/sessions');
        return http.Response(
          '[{"id":"session-1234567890","messageCount":2}]',
          200,
        );
      }),
    );

    await tester.pumpWidget(const MyApp());
    await tester.pumpAndSettle();

    expect(find.text('Ready to Chat'), findsOneWidget);
    expect(find.text('Open Chat'), findsOneWidget);
    expect(find.text('Start Fresh Session'), findsOneWidget);
    expect(find.text('Project Folder (Optional)'), findsOneWidget);
    expect(find.text('Connection Details'), findsOneWidget);

    await tester.tap(find.text('Connection Details'));
    await tester.pumpAndSettle();

    expect(find.textContaining('session-123456'), findsOneWidget);
    expect(find.textContaining('http://agent.test'), findsOneWidget);
  });

  testWidgets('shows a backend unavailable state', (tester) async {
    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
      client: MockClient((request) async {
        return http.Response('{"error":"server down"}', 500);
      }),
    );

    await tester.pumpWidget(const MyApp());
    await tester.pumpAndSettle();

    expect(
      find.textContaining('Could not reach the agent API'),
      findsOneWidget,
    );
    expect(find.text('Retry'), findsOneWidget);
  });

  testWidgets('explains empty workspace instead of silently doing nothing', (
    tester,
  ) async {
    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
      client: MockClient((request) async {
        expect(request.method, 'GET');
        return http.Response(
          '[{"id":"session-1234567890","messageCount":0}]',
          200,
        );
      }),
    );

    await tester.pumpWidget(const MyApp());
    await tester.pumpAndSettle();

    await tester.tap(find.text('Project Folder (Optional)'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Apply Workspace'));
    await tester.pumpAndSettle();

    expect(find.textContaining('Workspace is optional'), findsOneWidget);
  });

  testWidgets('start fresh session opens chat immediately', (tester) async {
    var requestCount = 0;

    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
      client: MockClient((request) async {
        requestCount++;
        if (requestCount == 1) {
          expect(request.method, 'GET');
          expect(request.url.path, '/sessions');
          return http.Response('[]', 200);
        }
        expect(request.method, 'POST');
        expect(request.url.path, '/sessions');
        return http.Response('{"sessionId":"fresh-session"}', 201);
      }),
    );

    await tester.pumpWidget(const MyApp());
    await tester.pumpAndSettle();

    await tester.tap(find.text('Start Fresh Session'));
    await tester.pumpAndSettle();

    expect(find.byType(DetailScreen), findsOneWidget);
  });

  testWidgets('sends chat text and renders streamed answer', (tester) async {
    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
      client: MockClient((request) async {
        expect(request.method, 'POST');
        expect(request.url.path, '/sessions/session-1/stream');
        expect(request.body, contains('hello agent'));
        return http.Response(
          'event: done\n'
          'data: {"type":"done","answer":"Hello from agent"}\n\n'
          'event: close\n'
          'data: {}\n\n',
          200,
          headers: {'content-type': 'text/event-stream'},
        );
      }),
    );

    await tester.pumpWidget(
      const MaterialApp(home: DetailScreen(sessionId: 'session-1')),
    );

    await tester.enterText(find.byType(TextField), 'hello agent');
    await tester.tap(find.byTooltip('Send'));
    await tester.pumpAndSettle();

    expect(find.text('hello agent'), findsOneWidget);
    expect(find.text('Hello from agent'), findsOneWidget);
  });

  testWidgets('approval bar shows action details and awaits decision', (
    tester,
  ) async {
    bool? approved;

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: ApprovalBar(
            action: const PendingAction(
              tool: 'write_file',
              risk: 'write',
              summary: 'Create file README.md',
              preview: 'File: README.md\n--- after\nhello',
            ),
            onDecision: (value) async {
              approved = value;
            },
          ),
        ),
      ),
    );

    expect(find.text('Create file README.md'), findsOneWidget);
    expect(find.text('write'), findsOneWidget);
    expect(find.textContaining('File: README.md'), findsOneWidget);

    await tester.tap(find.text('Approve'));
    await tester.pumpAndSettle();

    expect(approved, isTrue);
  });
}
