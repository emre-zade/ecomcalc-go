//go:build !windows
package main

// Bu fonksiyon Android'de çağrıldığında hiçbir şey yapmayacak
// Böylece program hata vermeden derlenecek.
func setWindowsAlwaysOnTop(title string, on bool) {
	// Boş bırakıyoruz
}