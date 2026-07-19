import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutterapp/Session/presentation/view/detail_screen.dart';
import 'package:flutterapp/Session/presentation/view/widgets/approval_bar.dart';
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

  testWidgets('shows the project hub actions and recent projects', (
    tester,
  ) async {
    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
      client: MockClient((request) async {
        expect(request.method, 'GET');
        expect(request.url.path, '/sessions');
        return http.Response(
          '[{"id":"session-1234567890","workspace":"/Users/me/alpha","messageCount":2}]',
          200,
        );
      }),
    );

    await tester.pumpWidget(const MyApp());
    await tester.pumpAndSettle();

    expect(find.text('Choose a project'), findsOneWidget);
    expect(find.text('New Project'), findsOneWidget);
    expect(find.text('Recent projects'), findsOneWidget);
    expect(find.text('alpha'), findsOneWidget);
    expect(find.text('/Users/me/alpha'), findsOneWidget);
  });

  testWidgets('shows a backend unavailable state on the hub', (tester) async {
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

  testWidgets('new project opens the backend folder browser', (tester) async {
    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
      client: MockClient((request) async {
        if (request.url.path == '/sessions') {
          return http.Response('[]', 200);
        }
        if (request.url.path == '/workspace/roots') {
          return http.Response(
            '{"roots":[{"path":"/Users/me","name":"me","kind":"home"}]}',
            200,
          );
        }
        if (request.url.path == '/workspace/browse') {
          return http.Response(
            '{"path":"/Users/me","roots":[{"path":"/Users/me","name":"me","kind":"home"}],"entries":[{"name":"alpha","path":"/Users/me/alpha","isDir":true}]}',
            200,
          );
        }
        return http.Response('{"error":"unexpected"}', 404);
      }),
    );

    await tester.pumpWidget(const MyApp());
    await tester.pumpAndSettle();

    await tester.tap(find.text('New Project'));
    await tester.pumpAndSettle();

    expect(find.text('Create Project Here'), findsOneWidget);
    expect(find.text('alpha'), findsOneWidget);
    expect(find.text('/Users/me'), findsOneWidget);
  });

  testWidgets('loads project history and sends streamed chat text', (
    tester,
  ) async {
    ApiService.configureForTesting(
      baseUriOverride: Uri.parse('http://agent.test'),
      client: MockClient((request) async {
        if (request.method == 'GET' &&
            request.url.path == '/sessions/session-1') {
          return http.Response(
            '{"id":"session-1","sessionId":"session-1","messageCount":2,"workspace":"/Users/me/alpha","messages":[{"role":"user","content":"previous question"},{"role":"assistant","content":"previous answer"}]}',
            200,
          );
        }
        if (request.method == 'POST' &&
            request.url.path == '/sessions/session-1/stream') {
          expect(request.body, contains('hello agent'));
          return http.Response(
            'event: done\n'
            'data: {"type":"done","answer":"Hello from agent"}\n\n'
            'event: close\n'
            'data: {}\n\n',
            200,
            headers: {'content-type': 'text/event-stream'},
          );
        }
        return http.Response('{"error":"unexpected"}', 404);
      }),
    );

    await tester.pumpWidget(
      const MaterialApp(home: DetailScreen(sessionId: 'session-1')),
    );
    await tester.pumpAndSettle();

    expect(find.text('alpha'), findsOneWidget);
    expect(find.text('previous question'), findsOneWidget);
    expect(find.text('previous answer'), findsOneWidget);

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
