import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:webview_flutter/webview_flutter.dart';

/// Android wrapper anti-cheat untuk e-learning.
///
/// [Aturan #3 master plan]
/// - FLAG_SECURE            : diset native di MainActivity.kt (blokir screenshot
///                            & screen recording), TIDAK bisa diakali dari JS.
/// - Deteksi AppLifeycle   : saat siswa menekan Home/Recent, `didChangeAppLifecycleState`
///                            memanggil MethodChannel "pingBackend" → backend mencatat
///                            sinyal "onPause" (kolom cheat_signals).
///
/// Komponen ini hanyalah WebView yang menunjuk ke URL frontend Next.js di Railway;
/// seluruh beban server cukup di REST API (SPA client-side render).

const String _baseUrl = String.fromEnvironment(
  'BACKEND_URL',
  defaultValue: 'https://elearning.example.up.railway.app',
);

const MethodChannel _channel = MethodChannel('elearning.anticheat');

void main() => runApp(const ElearningApp());

class ElearningApp extends StatefulWidget {
  const ElearningApp({super.key});

  @override
  State<ElearningApp> createState() => _ElearningAppState();
}

class _ElearningAppState extends State<ElearningApp> with WidgetsBindingObserver {
  late final WebViewController _controller;
  bool _everPaused = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setBackgroundColor(const Color(0xfff8f9fb))
      ..loadRequest(Uri.parse(_baseUrl));
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  /// Deteksi hilang fokus (mirip OnPause native). Kirim sinyal peringatan ke
  /// backend. Disengaja hanya sekali per sesi ujian utk mengurangi noise.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.paused && !_everPaused) {
      _everPaused = true;
      unawaited(_sendPauseSignal());
    }
  }

  Future<void> _sendPauseSignal() async {
    try {
      await _channel.invokeMethod<bool>('pingBackend');
    } catch (_) {
      // non-blocking; kegagalan tak mengganggu ujian
    }
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'E-Learning SMK',
      home: Scaffold(
        body: SafeArea(child: WebViewWidget(controller: _controller)),
      ),
    );
  }
}