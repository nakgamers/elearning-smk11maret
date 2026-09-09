package com.sekolah.elearning

import android.app.Activity
import android.os.Bundle
import android.view.WindowManager
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

/**
 * [Aturan #3 master plan] MainActivity native:
 *  - setFlag(FLAG_SECURE) : memblokir screenshot & screen recording di ujian.
 *  - onPause()            : deteksi siswa menekan Home/Recent → kirim sinyal
 *                           "onPause" ke backend (didocokan lewat MethodChannel).
 *
 * Logika WebView & lifecycle ada di lib/main.dart; kelas ini hanya mengamankan
 * jendela secara native (tidak bisa dirusak dari JS).
 */
class MainActivity : FlutterActivity() {
    private val CHANNEL = "elearning.anticheat"

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        // Selalu aktif (bukan cuma saat ujian): mencegah kebocoran soal.
        window.setFlags(
            WindowManager.LayoutParams.FLAG_SECURE,
            WindowManager.LayoutParams.FLAG_SECURE
        )
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "pingBackend" -> {
                        // Sinyal hanya bisa ditembak dari sisi native (onPause).
                        notifyBackendOnPause()
                        result.success(true)
                    }
                    else -> result.notImplemented()
                }
            }
    }

    /** Dipanggil saat aplikasi kehilangan fokus (tekan Home / buka Recent Apps). */
    override fun onPause() {
        super.onPause()
        notifyBackendOnPause()
    }

    private fun notifyBackendOnPause() {
        // Base URL frontend diisi lewat build-config; fallback local.
        val base = buildConfigField("BACKEND_URL") ?: "http://192.168.1.10:8080"
        Thread {
            try {
                val conn = java.net.URL("$base/api/cheat-signals").openConnection() as java.net.HttpURLConnection
                conn.requestMethod = "POST"
                conn.setRequestProperty("Content-Type", "application/json")
                conn.doOutput = true
                // Token siswa disimpan flutter; di sini cukup kirim event onPause.
                conn.outputStream.write("""{"jenis":"onPause"}""".toByteArray())
                conn.inputStream.close()
            } catch (_: Exception) {
                // non-blocking: kegagalan tak boleh mengganggu ujian
            }
        }.start()
    }

    private fun buildConfigField(key: String): String? {
        return try {
            val f = javaClass.getDeclaredField("BUILD_CONFIG")
            f.isAccessible = true
            val bc = f.get(null)
            val field = bc.javaClass.getField(key)
            field.get(bc) as? String
        } catch (_: Exception) { null }
    }
}