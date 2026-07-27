# Aturan ProGuard/R8 untuk build rilis Inventra Mobile.
# getDefaultProguardFile("proguard-android-optimize.txt") sudah mencakup aturan
# umum Android + Flutter; file ini menambah keep-rules untuk kelas yang diakses
# via refleksi/native yang bisa keliru dibuang R8.

# --- Flutter deferred components / Play Core ---
# Engine Flutter mereferensikan kelas Play Core walau fitur split-install tidak
# dipakai; tanpa ini R8 fullmode gagal dengan "Missing class".
-dontwarn com.google.android.play.core.**
-keep class com.google.android.play.core.** { *; }

# --- Google ML Kit barcode scanning (mobile_scanner) ---
# Model & detektor dimuat via refleksi; jangan dibuang/di-obfuscate.
-keep class com.google.mlkit.** { *; }
-dontwarn com.google.mlkit.**

# --- image_picker / androidx ---
-dontwarn androidx.**

# Pertahankan atribut yang dibutuhkan serialisasi/refleksi & stack trace jelas.
-keepattributes *Annotation*, Signature, InnerClasses, EnclosingMethod, SourceFile, LineNumberTable
