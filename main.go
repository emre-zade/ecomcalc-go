package main

import (
	"fmt"
	"go/token"
	"go/types"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ===== WINDOWS İÇİN ÖZEL TANIMLAMALAR =====
var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procFindWindowW  = user32.NewProc("FindWindowW")
	procSetWindowPos = user32.NewProc("SetWindowPos")
)

const (
	wHwndTopmost   int32 = -1
	wHwndNoTopmost int32 = -2
	wSwpNosize           = 0x0001
	wSwpNomove           = 0x0002
)

func setWindowsAlwaysOnTop(title string, on bool) {
	ptrTitle, _ := syscall.UTF16PtrFromString(title)
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(ptrTitle)))
	if hwnd == 0 {
		return
	}
	var hwndInsertAfter int32 = wHwndNoTopmost
	if on {
		hwndInsertAfter = wHwndTopmost
	}
	procSetWindowPos.Call(
		hwnd,
		uintptr(hwndInsertAfter),
		0, 0, 0, 0,
		uintptr(wSwpNomove|wSwpNosize),
	)
}

// ===== HESAPLAMA YAPISI =====
type Calc struct {
	Alis          float64
	Satis         float64
	Kargo         float64
	KomisyonP     float64
	Carpan        float64
	AutoFill      bool
	UseTargetMarg bool
	TargetMargin  float64
}

func (c *Calc) KomisyonTL() float64 { return c.Satis * c.KomisyonP / 100.0 }
func (c *Calc) KarTL() float64      { return c.Satis - c.Alis - c.Kargo - c.KomisyonTL() }
func (c *Calc) MarjYuzde() float64 {
	if c.Satis <= 0 {
		return 0
	}
	return c.KarTL() / c.Satis * 100.0
}

// ===== YARDIMCI FONKSİYONLAR =====
func replaceCommaDot(s string) string { return strings.ReplaceAll(s, ",", ".") }
func format0(f float64) string        { return fmt.Sprintf("%.0f", f) }

func evalExpr(s string) (float64, bool) {
	s = replaceCommaDot(s)
	fs := token.NewFileSet()
	tv, err := types.Eval(fs, nil, token.NoPos, s)
	if err != nil {
		return 0, false
	}
	val, err := strconv.ParseFloat(tv.Value.String(), 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

func parseMathOrFloat(s string) float64 {
	if v, ok := evalExpr(s); ok {
		return v
	}
	f, err := strconv.ParseFloat(replaceCommaDot(strings.TrimSpace(s)), 64)
	if err == nil {
		return f
	}
	return 0
}

// PAZARYERİ HESAPLAMA (YENİ)
func calculateMarketPrice(base float64, input string) float64 {
	input = strings.TrimSpace(input)
	if len(input) < 2 {
		return base
	}
	op := input[0]
	valStr := replaceCommaDot(input[1:])
	val, _ := strconv.ParseFloat(valStr, 64)

	switch op {
	case '+':
		return base + val
	case '-':
		return base - val
	case '*':
		return base * val
	case '/':
		if val != 0 {
			return base / val
		}
	}
	return base
}

const (
	pAlis          = "Alis"
	pSatis         = "Satis"
	pKargo         = "Kargo"
	pKomisyon      = "KomisyonP"
	pCarpan        = "Carpan"
	pAuto          = "AutoFill"
	pDark          = "DarkTheme"
	pUseTargetMarg = "UseTargetMargin"
	pTargetMargin  = "TargetMargin"
	pAlwaysOnTop   = "AlwaysOnTop"
	windowTitle    = "E-Ticaret Hesaplayıcı"

	// Yeni Prefs
	pHBFormula    = "HBFormula"
	pPTTFormula   = "PTTFormula"
	pPazarFormula = "PazarFormula"
)

func main() {
	a := app.NewWithID("com.solidmarket.ecomcalc")
	if a.Preferences().BoolWithFallback(pDark, false) {
		a.Settings().SetTheme(theme.DarkTheme())
	} else {
		a.Settings().SetTheme(theme.LightTheme())
	}

	w := a.NewWindow(windowTitle)
	w.Resize(fyne.NewSize(280, 560))

	calc := &Calc{}
	uiUpdating := false

	// Girdiler
	alisEntry := widget.NewEntry()
	alisPasteBtn := widget.NewButtonWithIcon("", theme.ContentPasteIcon(), func() {
		alisEntry.SetText(w.Clipboard().Content())
	})
	satisEntry := widget.NewEntry()
	satisCopyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(satisEntry.Text)
	})
	kargoEntry := widget.NewEntry()
	komisyonEntry := widget.NewEntry()
	carpanEntry := widget.NewEntry()
	targetMarginEntry := widget.NewEntry()

	// Pazaryeri Girdileri (YENİ)
	hbFormula := widget.NewEntry()
	hbFormula.SetPlaceHolder("Örn: *1.05")
	hbResult := widget.NewEntry()

	pttFormula := widget.NewEntry()
	pttFormula.SetPlaceHolder("Örn: +15")
	pttResult := widget.NewEntry()

	pazarFormula := widget.NewEntry()
	pazarFormula.SetPlaceHolder("Örn: *0.95")
	pazarResult := widget.NewEntry()

	komisyonLabel := widget.NewLabel("0 TL")
	karRT := widget.NewRichText(&widget.TextSegment{Text: "0 TL"})
	marjLabel := widget.NewLabel("0 %")

	recalc := func() {
		if uiUpdating {
			return
		}
		calc.Alis = parseMathOrFloat(alisEntry.Text)
		calc.Kargo = parseMathOrFloat(kargoEntry.Text)
		calc.KomisyonP = parseMathOrFloat(komisyonEntry.Text)

		if calc.AutoFill {
			if calc.UseTargetMarg {
				calc.TargetMargin = parseMathOrFloat(targetMarginEntry.Text)
				totalRate := (calc.KomisyonP + calc.TargetMargin) / 100.0
				if totalRate >= 1.0 {
					calc.Satis = 0
				} else {
					calc.Satis = (calc.Alis + calc.Kargo) / (1.0 - totalRate)
				}
			} else {
				calc.Carpan = parseMathOrFloat(carpanEntry.Text)
				maliyet := (calc.Alis * calc.Carpan) + calc.Kargo
				calc.Satis = (maliyet * (calc.KomisyonP / 100)) + maliyet
			}
			uiUpdating = true
			satisEntry.SetText(format0(calc.Satis))
			uiUpdating = false
		} else {
			calc.Satis = parseMathOrFloat(satisEntry.Text)
		}

		// UI GÜNCELLEME
		kar := calc.KarTL()
		seg := &widget.TextSegment{Text: fmt.Sprintf("%s TL", format0(kar))}
		if kar >= 0 {
			seg.Style = widget.RichTextStyle{ColorName: theme.ColorNameSuccess}
		} else {
			seg.Style = widget.RichTextStyle{ColorName: theme.ColorNameError}
		}
		karRT.Segments = []widget.RichTextSegment{seg}
		karRT.Refresh()
		komisyonLabel.SetText(format0(calc.KomisyonTL()) + " TL")
		marjLabel.SetText(format0(calc.MarjYuzde()) + " %")

		// Pazaryeri Güncelleme (YENİ)
		hbResult.SetText(format0(calculateMarketPrice(calc.Satis, hbFormula.Text)))
		pttResult.SetText(format0(calculateMarketPrice(calc.Satis, pttFormula.Text)))
		pazarResult.SetText(format0(calculateMarketPrice(calc.Satis, pazarFormula.Text)))

		savePrefs(a, calc, alisEntry, satisEntry, kargoEntry, komisyonEntry, carpanEntry, targetMarginEntry, hbFormula, pttFormula, pazarFormula)
	}

	// Eventler
	entries := []*widget.Entry{alisEntry, satisEntry, kargoEntry, komisyonEntry, carpanEntry, targetMarginEntry, hbFormula, pttFormula, pazarFormula}
	for _, e := range entries {
		e.OnChanged = func(s string) { recalc() }
	}

	// Pazaryeri Paneli (YENİ)
	copyBtn := func(e *widget.Entry) *widget.Button {
		return widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() { w.Clipboard().SetContent(e.Text) })
	}
	marketGrid := container.NewVBox(
		widget.NewSeparator(),
		container.NewGridWithColumns(3, widget.NewLabel("Hepsiburada:"), hbFormula, container.NewBorder(nil, nil, nil, copyBtn(hbResult), hbResult)),
		container.NewGridWithColumns(3, widget.NewLabel("PttAVM:"), pttFormula, container.NewBorder(nil, nil, nil, copyBtn(pttResult), pttResult)),
		container.NewGridWithColumns(3, widget.NewLabel("Pazarama:"), pazarFormula, container.NewBorder(nil, nil, nil, copyBtn(pazarResult), pazarResult)),
	)
	marketGrid.Hide()

	expandBtn := widget.NewButtonWithIcon("Pazaryerlerini Göster", theme.MenuExpandIcon(), func() {
		if marketGrid.Visible() {
			marketGrid.Hide()
			w.Resize(fyne.NewSize(280, 560))
		} else {
			marketGrid.Show()
			w.Resize(fyne.NewSize(330, 685))
		}
	})

	// Checkboxlar
	autoFillCheck := widget.NewCheck("Otomatik Hesapla", func(b bool) { calc.AutoFill = b; recalc() })
	useTargetMargCheck := widget.NewCheck("Hedef Marjı Kullan", func(b bool) { calc.UseTargetMarg = b; recalc() })
	alwaysOnTopCheck := widget.NewCheck("Her Zaman Üstte", func(b bool) {
		a.Preferences().SetBool(pAlwaysOnTop, b)
		if runtime.GOOS == "windows" {
			setWindowsAlwaysOnTop(windowTitle, b)
		}
	})

	// Layout
	form := container.New(layout.NewFormLayout(),
		widget.NewLabel("Alış (TL):"),
		container.NewBorder(nil, nil, nil, alisPasteBtn, alisEntry), // Sağda Yapıştır butonu

		widget.NewLabel("Baz Satış (TL):"),
		container.NewBorder(nil, nil, nil, satisCopyBtn, satisEntry), // Sağda Kopyala butonu

		widget.NewLabel("Kargo (TL):"), kargoEntry,
		widget.NewLabel("Komisyon (%):"), komisyonEntry,
		widget.NewLabel("Çarpan (x):"), carpanEntry,
		widget.NewLabel("Hedef Marj (%):"), targetMarginEntry,
		widget.NewLabel("Kâr:"), karRT,
		widget.NewLabel("Komisyon:"), komisyonLabel,
		widget.NewLabel("Marj:"), marjLabel,
	)

	uiUpdating = true
	loadPrefs(a, calc, alisEntry, satisEntry, kargoEntry, komisyonEntry, carpanEntry, targetMarginEntry, hbFormula, pttFormula, pazarFormula, autoFillCheck, useTargetMargCheck, alwaysOnTopCheck)
	uiUpdating = false

	content := container.NewVScroll(container.NewVBox(
		form,
		widget.NewSeparator(),
		autoFillCheck, useTargetMargCheck, alwaysOnTopCheck,
		expandBtn,
		marketGrid,
		widget.NewButton("Tema Değiştir", func() {
			dark := !a.Preferences().BoolWithFallback(pDark, false)
			a.Preferences().SetBool(pDark, dark)
			if dark {
				a.Settings().SetTheme(theme.DarkTheme())
			} else {
				a.Settings().SetTheme(theme.LightTheme())
			}
		}),
	))

	w.SetContent(content)

	// Başlangıç "Always on Top" kontrolü
	if runtime.GOOS == "windows" && a.Preferences().BoolWithFallback(pAlwaysOnTop, false) {
		go func() {
			time.Sleep(time.Millisecond * 500)
			setWindowsAlwaysOnTop(windowTitle, true)
		}()
	}

	recalc()
	w.ShowAndRun()
}

func loadPrefs(a fyne.App, c *Calc, alis, satis, kargo, komisyon, carpan, targetMargin, hb, ptt, pazar *widget.Entry, auto, target, top *widget.Check) {
	p := a.Preferences()

	alis.SetText(p.StringWithFallback(pAlis, ""))
	satis.SetText(p.StringWithFallback(pSatis, ""))
	kargo.SetText(p.StringWithFallback(pKargo, ""))
	komisyon.SetText(p.StringWithFallback(pKomisyon, "15"))
	carpan.SetText(p.StringWithFallback(pCarpan, "1"))
	targetMargin.SetText(p.StringWithFallback(pTargetMargin, "30"))

	hb.SetText(p.StringWithFallback(pHBFormula, "*1"))
	ptt.SetText(p.StringWithFallback(pPTTFormula, "*1"))
	pazar.SetText(p.StringWithFallback(pPazarFormula, "*1"))

	auto.SetChecked(p.BoolWithFallback(pAuto, false))
	target.SetChecked(p.BoolWithFallback(pUseTargetMarg, false))
	top.SetChecked(p.BoolWithFallback(pAlwaysOnTop, false))

	c.AutoFill = auto.Checked
	c.UseTargetMarg = target.Checked
}

func savePrefs(a fyne.App, c *Calc, alis, satis, kargo, komisyon, carpan, targetMargin, hb, ptt, pazar *widget.Entry) {
	p := a.Preferences()
	p.SetString(pAlis, alis.Text)
	p.SetString(pSatis, satis.Text)
	p.SetString(pKargo, kargo.Text)
	p.SetString(pKomisyon, komisyon.Text)
	p.SetString(pCarpan, carpan.Text)
	p.SetString(pTargetMargin, targetMargin.Text)

	p.SetString(pHBFormula, hb.Text)
	p.SetString(pPTTFormula, ptt.Text)
	p.SetString(pPazarFormula, pazar.Text)

	p.SetBool(pAuto, c.AutoFill)
	p.SetBool(pUseTargetMarg, c.UseTargetMarg)
}
