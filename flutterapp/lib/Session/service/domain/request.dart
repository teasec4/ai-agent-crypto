class AskRequest {
  final String? sessionId;
  final String message;

  AskRequest({this.sessionId, required this.message});

  Map<String, dynamic> toJson() => {
    if (sessionId != null) 'sessionId': sessionId,
    'message': message,
  };
}
