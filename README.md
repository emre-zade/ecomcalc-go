
# EComCalc (Go/Fyne)

Basit e‑ticaret fiyat hesaplayıcı. Qt/QML sürümünden Go + [Fyne](https://fyne.io) ile taşınmıştır.

## Alanlar
- **Alış Fiyatı (TL)**
- **Satış Fiyatı (TL)** (Otomatik doldurma açık ise `Satış = Alış * Çarpan + Kargo`)
- **Kargo Fiyatı (TL)**
- **Komisyon Oranı (%)**
- **Çarpan Oranı (x)**
- Çıktılar: **Komisyon TL**, **Kar TL**, **Marj %**

> Not: İlk sürümde KDV ve gelişmiş formüller yok. İstersen ekleriz.

## Linux'ta Derleme (Ubuntu 24.04+)
```bash
sudo apt update
sudo apt install -y golang gcc libgl1-mesa-dev xorg-dev
cd ecomcalc-go
go mod tidy
go run .
go build -o ecomcalc
./ecomcalc
```

## Android APK Oluşturma (Fyne)
Fyne, Android için paketlemeyi destekler. Ön koşullar:
- Android Studio (SDK) ve NDK yüklü
- Ortam değişkenleri:
  - `ANDROID_SDK_ROOT`
  - `ANDROID_NDK_HOME` (veya `ANDROID_NDK_ROOT`)
  - `JAVA_HOME` (JDK 17 önerilir)

### Adımlar
```bash
# 1) Fyne CLI kur
go install fyne.io/fyne/v2/cmd/fyne@latest
# 2) Projeye geç
cd ecomcalc-go
# 3) Paketle (appID'yi değiştirebilirsin)
fyne package -os android -app-id com.solidmarket.ecomcalc -name "EComCalc"
# Çıktı: EComCalc.apk (bin/ klasörü)
```

> Alternatif: `fyne package -os android/arm64` ile mimari seçilebilir. Keystore oluşturmazsan debug imzalı APK çıkar.

## Yol Haritası
- KDV alanı ve `Satış = ...` formülü seçenekleri
- Komisyon platform preset'leri (Hepsiburada/Trendyol)
- JSON ile profil kaydetme/yükleme
- Pencere "Üstte tut" (platform uygunluğu kadar)
