package main

import (
	"fmt"
	"go/token"
	"go/types"
	"image/color"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ===== ÖZEL TEMA YAPISI (FONT KÜÇÜLTME) =====
type myTheme struct {
	fyne.Theme
}

func (m myTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 12 // Daha zarif görünüm için
	}
	if name == theme.SizeNameCaptionText {
		return 10
	}
	if name == theme.SizeNameInlineIcon {
		return 18
	}
	if name == theme.SizeNamePadding {
		return 4
	}
	return m.Theme.Size(name)
}

func applyCustomTheme(a fyne.App, dark bool) {
	if dark {
		a.Settings().SetTheme(&myTheme{Theme: theme.DarkTheme()})
	} else {
		a.Settings().SetTheme(&myTheme{Theme: theme.LightTheme()})
	}
}

type themeSize struct {
	Height float32
	Width  float32
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

	pHBFormula    = "HBFormula"
	pPTTFormula   = "PTTFormula"
	pPazarFormula = "PazarFormula"
)

func main() {
	a := app.NewWithID("com.solidmarket.ecomcalc")

	isDark := a.Preferences().BoolWithFallback(pDark, false)
	applyCustomTheme(a, isDark)

	myThemeSize := themeSize{
		Width:  258,
		Height: 545,
	}
	w := a.NewWindow(windowTitle)
	w.Resize(fyne.NewSize(myThemeSize.Width, myThemeSize.Height))

	calc := &Calc{}
	uiUpdating := false

	// Girdiler
	alisEntry := widget.NewEntry()
	alisEntry.SetPlaceHolder("Örn: 10+5")
	alisPasteBtn := widget.NewButtonWithIcon("", theme.ContentPasteIcon(), func() {
		alisEntry.SetText(w.Clipboard().Content())
	})

	// Alış Fiyatı Önizleme Etiketi (Gri ve Sola Hizalı)
	alisPreviewLabel := canvas.NewText("", color.NRGBA{128, 128, 128, 255})
	alisPreviewLabel.TextSize = 10

	satisEntry := widget.NewEntry()
	satisCopyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(satisEntry.Text)
	})
	kargoEntry := widget.NewEntry()
	komisyonEntry := widget.NewEntry()
	carpanEntry := widget.NewEntry()
	targetMarginEntry := widget.NewEntry()

	// Pazaryeri Girdileri
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

	// Input Aktif/Pasif Yönetimi
	updateInputStates := func() {
		if calc.AutoFill {
			if calc.UseTargetMarg {
				targetMarginEntry.Enable()
				carpanEntry.Disable()
			} else {
				targetMarginEntry.Disable()
				carpanEntry.Enable()
			}
		} else {
			targetMarginEntry.Disable()
			carpanEntry.Disable()
		}
	}

	recalc := func() {
		if uiUpdating {
			return
		}

		// Alış Fiyatı Matematik Önizlemesi - SOLA HİZALI MANTIK
		alisRaw := alisEntry.Text
		if val, ok := evalExpr(alisRaw); ok && strings.ContainsAny(alisRaw, "+-*/") {
			alisPreviewLabel.Text = "= " + format0(val)
		} else {
			alisPreviewLabel.Text = ""
		}
		alisPreviewLabel.Refresh()

		calc.Alis = parseMathOrFloat(alisEntry.Text)
		calc.Kargo = parseMathOrFloat(kargoEntry.Text)
		calc.KomisyonP = parseMathOrFloat(komisyonEntry.Text)

		if calc.AutoFill {
			if calc.UseTargetMarg {
				calc.TargetMargin = parseMathOrFloat(targetMarginEntry.Text)
				markupOrani := calc.TargetMargin / 100.0
				komisyonOrani := calc.KomisyonP / 100.0

				if komisyonOrani >= 1.0 {
					calc.Satis = 0
				} else {
					calc.Satis = (calc.Alis*(1.0+markupOrani) + calc.Kargo) / (1.0 - komisyonOrani)
				}
			} else {
				calc.Carpan = parseMathOrFloat(carpanEntry.Text)
				maliyet := (calc.Alis * calc.Carpan) + calc.Kargo
				komisyonOrani := calc.KomisyonP / 100.0

				if komisyonOrani >= 1.0 {
					calc.Satis = 0
				} else {
					calc.Satis = maliyet / (1.0 - komisyonOrani)
				}
			}
			uiUpdating = true
			satisEntry.SetText(format0(calc.Satis))
			uiUpdating = false
		} else {
			calc.Satis = parseMathOrFloat(satisEntry.Text)
		}

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

		hbResult.SetText(format0(calculateMarketPrice(calc.Satis, hbFormula.Text)))
		pttResult.SetText(format0(calculateMarketPrice(calc.Satis, pttFormula.Text)))
		pazarResult.SetText(format0(calculateMarketPrice(calc.Satis, pazarFormula.Text)))

		savePrefs(a, calc, alisEntry, satisEntry, kargoEntry, komisyonEntry, carpanEntry, targetMarginEntry, hbFormula, pttFormula, pazarFormula)
	}

	entries := []*widget.Entry{alisEntry, satisEntry, kargoEntry, komisyonEntry, carpanEntry, targetMarginEntry, hbFormula, pttFormula, pazarFormula}
	for _, e := range entries {
		e.OnChanged = func(s string) { recalc() }
	}

	fixWidth := func(entry *widget.Entry, width float32) fyne.CanvasObject {
		return container.NewGridWrap(fyne.NewSize(width, 34), entry)
	}

	copyBtn := func(e *widget.Entry) *widget.Button {
		return widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() { w.Clipboard().SetContent(e.Text) })
	}

	marketGrid := container.NewVBox(
		widget.NewSeparator(),
		container.New(layout.NewFormLayout(),
			widget.NewLabel("Hepsiburada:"),
			container.NewBorder(nil, nil, fixWidth(hbFormula, 60), copyBtn(hbResult), hbResult),
			widget.NewLabel("PttAVM:"),
			container.NewBorder(nil, nil, fixWidth(pttFormula, 60), copyBtn(pttResult), pttResult),
			widget.NewLabel("Pazarama:"),
			container.NewBorder(nil, nil, fixWidth(pazarFormula, 60), copyBtn(pazarResult), pazarResult),
		),
	)

	expandBtn := widget.NewButtonWithIcon("Pazaryerlerini Göster", theme.MenuExpandIcon(), func() {
		if marketGrid.Visible() {
			marketGrid.Hide()
			w.Resize(fyne.NewSize(myThemeSize.Width, myThemeSize.Height))
		} else {
			marketGrid.Show()
			w.Resize(fyne.NewSize(myThemeSize.Width, 665))
		}
	})

	autoFillCheck := widget.NewCheck("Otomatik Hesapla", func(b bool) {
		calc.AutoFill = b
		updateInputStates()
		recalc()
	})
	useTargetMargCheck := widget.NewCheck("Hedef Net Kâr Kullan", func(b bool) {
		calc.UseTargetMarg = b
		updateInputStates()
		recalc()
	})
	alwaysOnTopCheck := widget.NewCheck("Her Zaman Üstte", func(b bool) {
		a.Preferences().SetBool(pAlwaysOnTop, b)
		if runtime.GOOS == "windows" {
			setWindowsAlwaysOnTop(windowTitle, b)
		}
	})

	// Alış Girişini ve Önizleme Etiketini SOLA HİZALI olarak birleştiriyoruz
	alisGroup := container.NewVBox(
		container.NewBorder(nil, nil, nil, alisPasteBtn, alisEntry),
		container.NewHBox(alisPreviewLabel), // Artık Spacer yok, doğrudan sola yaslı
	)

	form := container.New(layout.NewFormLayout(),
		widget.NewLabel("Alış Fiyatı (TL):"), alisGroup,
		widget.NewLabel("Baz Satış (TL):"), container.NewBorder(nil, nil, nil, satisCopyBtn, satisEntry),
		widget.NewLabel("Kargo Ücreti (TL):"), kargoEntry,
		widget.NewLabel("Pazaryeri Kom. (%):"), komisyonEntry,
		widget.NewLabel("Maliyet Çarpanı (x):"), carpanEntry,
		widget.NewLabel("Hedef Net Kâr (%):"), targetMarginEntry,
		widget.NewLabel("Net Kâr (TL):"), karRT,
		widget.NewLabel("Komisyon (TL):"), komisyonLabel,
		widget.NewLabel("Satış Marjı (%):"), marjLabel,
	)

	uiUpdating = true
	loadPrefs(a, calc, alisEntry, satisEntry, kargoEntry, komisyonEntry, carpanEntry, targetMarginEntry, hbFormula, pttFormula, pazarFormula, autoFillCheck, useTargetMargCheck, alwaysOnTopCheck)
	updateInputStates()
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
			applyCustomTheme(a, dark)
		}),
	))

	w.SetContent(content)

	if runtime.GOOS == "windows" && a.Preferences().BoolWithFallback(pAlwaysOnTop, false) {
		go func() {
			time.Sleep(time.Millisecond * 500)
			setWindowsAlwaysOnTop(windowTitle, true)
		}()
	}

	marketGrid.Hide()

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
