# Android Anti-Cheat Wrapper (E-Learning)

WebView wrapper Flutter yang mengarah ke frontend Next.js di Railway, dengan
dua pengaman native sesuai **Aturan #3** master plan.

## Fitur keamanan
1. **FLAG_SECURE** — diset di `MainActivity.kt` baris `onCreate`. Memblokir
   screenshot & screen recording di seluruh aplikasi (bukan hanya halaman ujian,
   agar tidak ada celah kebocoran soal).
2. **Deteksi kehilangan fokus** — `didChangeAppLifecycleState(paused)` di
   `main.dart` memanggil `MethodChannel → pingBackend → onPause` yang kemudian
   menembak `POST /api/cheat-signals` (`{"jenis":"onPause"}`) dari sisi native.
   Backend mencatatnya di tabel `cheat_signals` untuk ditindaklanjuti pengawas.

## Cara build (butuh Flutter SDK di mesin ini)
```bash
flutter create . --platforms=android --org com.sekolah --project-name elearning_mobile
flutter pub add webview_flutter
# timpa lib/main.dart & MainActivity.kt & AndroidManifest.xml dengan file repo ini
flutter build apk --release --dart-define=BACKEND_URL=https://frontend-anda.up.railway.app
```
APK jadi: `build/app/outputs/flutter-apk/app-release.apk`.

> Catatan: bangun ulang struktur android/ via `flutter create` dulu, sebab repo
> ini hanya menyimpan 3 file inti (main.dart, MainActivity.kt, AndroidManifest.xml)
> — buildToolConfig gradle & resource lainnya dihasilkan otomatis oleh Flutter.
>
> BASE_URL di `main.dart` memakai `BACKEND_URL` dart-define (fallback default).
> Sesuaikan dengan domain Railway frontend Anda saat build.