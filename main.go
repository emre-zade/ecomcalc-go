package main

import (
	"fmt"
	"go/token"
	"go/types"
	"os/exec"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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

func replaceCommaDot(s string) string {
	r := []rune(s)
	for i := range r {
		if r[i] == ',' {
			r[i] = '.'
		}
	}
	return string(r)
}

func parseFloatStrict(s string) (float64, bool) {
	f, err := strconv.ParseFloat(replaceCommaDot(strings.TrimSpace(s)), 64)
	return f, err == nil
}

func hasOperator(s string) bool {
	s = strings.TrimSpace(s)
	for _, r := range s {
		if strings.ContainsRune("+-*/()", r) {
			return true
		}
	}
	return false
}

func format0(f float64) string { return fmt.Sprintf("%.0f", f) }

// Matematiksel ifade çözücü (örn: "100+12+5")
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
	if f, ok := parseFloatStrict(s); ok {
		return f
	}
	return 0
}

// ===== UYGULAMA DEĞİŞKENLERİ =====

var (
	uiUpdating    bool
	lastChanged   string
	komisyonLabel *widget.Label
	karRT         *widget.RichText
	marjLabel     *widget.Label
)

const (
	pAlis     = "Alis"
	pSatis    = "Satis"
	pKargo    = "Kargo"
	pKomisyon = "KomisyonP"
	pCarpan   = "Carpan"
	pAuto     = "AutoFill"
	pDark     = "DarkTheme"

	pUseTargetMarg = "UseTargetMargin"
	pTargetMargin  = "TargetMargin"

	pAlwaysOnTop = "AlwaysOnTop"
)

// ===== MAIN =====

func main() {
	a := app.NewWithID("com.solidmarket.ecomcalc")

	// Tema Ayarı
	if a.Preferences().BoolWithFallback(pDark, false) {
		a.Settings().SetTheme(theme.DarkTheme())
	} else {
		a.Settings().SetTheme(theme.LightTheme())
	}

	w := a.NewWindow("E-Ticaret Hesaplayıcı")
	w.Resize(fyne.NewSize(300, 550))

	// Her zaman üstte ayarı (Kaydedilmiş tercih)
	isAlwaysOnTop := a.Preferences().BoolWithFallback(pAlwaysOnTop, false)
	if isAlwaysOnTop {
		//w.SetOnTop(true)
		// Linux hack'i pencere gösterildikten sonra çalışır, aşağıda tekrar çağıracağız
	}

	calc := &Calc{Carpan: 1.0, KomisyonP: 15.0, AutoFill: false, TargetMargin: 30}

	// --- Girdiler ---
	alisEntry := widget.NewEntry()
	alisEntry.SetPlaceHolder("(örn: 100+10)")
	satisEntry := widget.NewEntry()
	satisEntry.SetPlaceHolder("0")
	kargoEntry := widget.NewEntry()
	kargoEntry.SetPlaceHolder("rakam yaz")
	komisyonEntry := widget.NewEntry()
	komisyonEntry.SetPlaceHolder("rakam yaz")
	carpanEntry := widget.NewEntry()
	carpanEntry.SetPlaceHolder("rakam yaz")
	targetMarginEntry := widget.NewEntry()
	targetMarginEntry.SetPlaceHolder("rakam yaz")

	// --- Etiket Oluşturucular ---
	mkLabel := func(text string) *canvas.Text {
		t := canvas.NewText(text, theme.ForegroundColor())
		t.TextSize = theme.TextSize()
		return t
	}
	lblAlis := mkLabel("Alış Fiyatı (TL):")
	lblSatis := mkLabel("Satış Fiyatı (TL):")
	lblKargo := mkLabel("Kargo Fiyatı (TL):")
	lblKomisIn := mkLabel("Komisyon Oranı (%):")
	lblCarpan := mkLabel("Çarpan Oranı (x):")
	lblTarget := mkLabel("Hedef Marj (%):")
	lblKar := mkLabel("Kâr:")
	lblKomisOut := mkLabel("Komisyon Tutarı:")
	lblMarj := mkLabel("Kâr Marjı:")

	alisHint := canvas.NewText("", theme.DisabledColor())
	alisHint.TextSize = theme.TextSize() - 1
	alisHint.Hide()

	komisyonLabel = widget.NewLabel("0 TL")
	karRT = widget.NewRichText(&widget.TextSegment{
		Text:  "0 TL",
		Style: widget.RichTextStyle{ColorName: theme.ColorNameSuccess},
	})
	marjLabel = widget.NewLabel("0 %")

	// --- Kontroller ---
	autoFillCheck := widget.NewCheck("Satışı Otomatik Hesapla", nil)
	useTargetMargCheck := widget.NewCheck("Hedef Marjı Kullan", nil)
	alwaysOnTopCheck := widget.NewCheck("Her Zaman Üstte", nil)

	// --- Tercihleri Yükle ---
	loadPrefs(a, calc, alisEntry, satisEntry, kargoEntry, komisyonEntry, carpanEntry, targetMarginEntry)
	alwaysOnTopCheck.SetChecked(isAlwaysOnTop)

	// --- UI Yardımcıları ---
	applyUIState := func() {
		lblCarpan.Color = theme.ForegroundColor()
		lblTarget.Color = theme.ForegroundColor()

		if !calc.AutoFill {
			carpanEntry.Disable()
			targetMarginEntry.Disable()
			lblCarpan.Color = theme.DisabledColor()
			lblTarget.Color = theme.DisabledColor()
		} else {
			if calc.UseTargetMarg {
				targetMarginEntry.Enable()
				carpanEntry.Disable()
				lblTarget.Color = theme.ForegroundColor()
				lblCarpan.Color = theme.DisabledColor()
			} else {
				targetMarginEntry.Disable()
				carpanEntry.Enable()
				lblCarpan.Color = theme.ForegroundColor()
				lblTarget.Color = theme.DisabledColor()
			}
		}
		lblCarpan.Refresh()
		lblTarget.Refresh()
	}

	refreshOutputs := func(kar, komisyon, marj float64) {
		seg := &widget.TextSegment{Text: fmt.Sprintf("%s TL", format0(kar))}
		if kar >= 0 {
			seg.Style = widget.RichTextStyle{ColorName: theme.ColorNameSuccess}
		} else {
			seg.Style = widget.RichTextStyle{ColorName: theme.ColorNameError}
		}
		karRT.Segments = []widget.RichTextSegment{seg}
		karRT.Refresh()

		komisyonLabel.SetText(format0(komisyon) + " TL")
		komisyonLabel.Refresh()

		marjLabel.SetText(format0(marj) + " %")
		marjLabel.Refresh()
	}

	// --- Ana Hesaplama Fonksiyonu ---
	recalc := func() {
		if uiUpdating {
			return
		}

		// 1. Verileri Oku
		calc.Alis = parseMathOrFloat(alisEntry.Text)
		calc.Kargo = parseMathOrFloat(kargoEntry.Text)
		calc.KomisyonP = parseMathOrFloat(komisyonEntry.Text)

		// 2. Alış Fiyatı İpucu
		if hasOperator(alisEntry.Text) {
			alisHint.Text = fmt.Sprintf("≈ %s TL", format0(calc.Alis))
			alisHint.Show()
		} else {
			alisHint.Hide()
		}
		alisHint.Refresh()

		// 3. Otomatik Satış Fiyatı Hesaplama
		if calc.AutoFill {
			if calc.UseTargetMarg {
				// Marj formülü: Satis = (Maliyet) / (1 - (Komisyon + Marj))
				calc.TargetMargin = parseMathOrFloat(targetMarginEntry.Text)
				totalRate := (calc.KomisyonP + calc.TargetMargin) / 100.0

				if totalRate >= 1.0 {
					calc.Satis = 0 // Hatalı oran
				} else {
					calc.Satis = (calc.Alis + calc.Kargo) / (1.0 - totalRate)
				}
			} else {
				// Çarpan formülü
				calc.Carpan = parseMathOrFloat(carpanEntry.Text)
				maliyet := (calc.Alis * calc.Carpan) + calc.Kargo
				komisyon := maliyet * (calc.KomisyonP / 100)
				calc.Satis = komisyon + maliyet
			}

			// Hesaplanan değeri kutucuğa yaz (loop oluşmaması için flag kullanabilirdik ama SetText OnChanged tetikler, basitçe en sona bırakıyoruz)
			uiUpdating = true
			satisEntry.SetText(format0(calc.Satis))
			uiUpdating = false
		} else {
			// Manuel giriş
			calc.Satis = parseMathOrFloat(satisEntry.Text)
		}

		// 4. Çıktıları Hesapla
		k := calc.KarTL()
		cVal := calc.KomisyonTL()
		m := calc.MarjYuzde()

		refreshOutputs(k, cVal, m)
		savePrefs(a, calc, alisEntry, satisEntry, kargoEntry, komisyonEntry, carpanEntry, targetMarginEntry)
	}

	// --- Event Handler'lar (Eksik olan kısım burasıydı) ---
	alisEntry.OnChanged = func(s string) { recalc() }
	satisEntry.OnChanged = func(s string) { recalc() }
	kargoEntry.OnChanged = func(s string) { recalc() }
	komisyonEntry.OnChanged = func(s string) { recalc() }
	carpanEntry.OnChanged = func(s string) { recalc() }
	targetMarginEntry.OnChanged = func(s string) { recalc() }

	autoFillCheck.OnChanged = func(b bool) {
		calc.AutoFill = b
		recalc()
		applyUIState()
	}

	useTargetMargCheck.OnChanged = func(b bool) {
		calc.UseTargetMarg = b
		recalc()
		applyUIState()
	}

	alwaysOnTopCheck.OnChanged = func(b bool) {
		a.Preferences().SetBool(pAlwaysOnTop, b)
		//w.SetOnTop(b)
		setLinuxAlwaysOnTop(w.Title(), b)
	}

	// --- Tema Butonu ---
	themeBtn := widget.NewButton("Tema Değiştir", func() {
		dark := !a.Preferences().BoolWithFallback(pDark, false)
		a.Preferences().SetBool(pDark, dark)
		if dark {
			a.Settings().SetTheme(theme.DarkTheme())
		} else {
			a.Settings().SetTheme(theme.LightTheme())
		}
		// Renkleri yenile
		lbls := []*canvas.Text{lblAlis, lblSatis, lblKargo, lblKomisIn, lblCarpan, lblTarget, lblKar, lblKomisOut, lblMarj}
		for _, t := range lbls {
			t.Color = theme.ForegroundColor()
			t.Refresh()
		}
		applyUIState()
	})

	// --- Layout ---
	alisCell := container.NewVBox(alisEntry, alisHint)
	formGrid := container.New(
		layout.NewFormLayout(),
		lblAlis, alisCell,
		lblSatis, satisEntry,
		lblKargo, kargoEntry,
		lblKomisIn, komisyonEntry,
		lblCarpan, carpanEntry,
		lblTarget, targetMarginEntry,
		lblKar, karRT,
		lblKomisOut, komisyonLabel,
		lblMarj, marjLabel,
	)

	buttons := container.NewVBox(autoFillCheck, useTargetMargCheck, alwaysOnTopCheck, widget.NewSeparator(), themeBtn)
	content := container.NewVBox(formGrid, widget.NewSeparator(), buttons)
	w.SetContent(content)

	// Başlangıç durumu
	uiUpdating = true
	applyUIState()
	useTargetMargCheck.SetChecked(calc.UseTargetMarg)
	autoFillCheck.SetChecked(calc.AutoFill)
	uiUpdating = false

	// Linux için "Her Zaman Üstte" özelliğini pencere açıldıktan hemen sonra tetikle
	go func() {
		if isAlwaysOnTop {
			setLinuxAlwaysOnTop(w.Title(), true)
		}
	}()

	recalc() // İlk hesaplama
	w.ShowAndRun()
}

// ---------------- Linux Helper (wmctrl) ----------------

func setLinuxAlwaysOnTop(winTitle string, on bool) {
	cmdPath, err := exec.LookPath("wmctrl")
	if err != nil {
		return
	}
	var mode string
	if on {
		mode = "add,above"
	} else {
		mode = "remove,above"
	}
	// wmctrl -r "title" -b add,above
	exec.Command(cmdPath, "-r", winTitle, "-b", mode).Run()
}

// ---------------- Prefs Helpers ----------------

func loadPrefs(a fyne.App, c *Calc, alis, satis, kargo, komisyon, carpan, targetMargin *widget.Entry) {
	p := a.Preferences()
	alis.SetText(p.StringWithFallback(pAlis, ""))
	satis.SetText(p.StringWithFallback(pSatis, ""))
	kargo.SetText(p.StringWithFallback(pKargo, ""))
	komisyon.SetText(p.StringWithFallback(pKomisyon, "15"))
	carpan.SetText(p.StringWithFallback(pCarpan, "1"))
	targetMargin.SetText(p.StringWithFallback(pTargetMargin, "30"))

	c.AutoFill = p.BoolWithFallback(pAuto, false)
	c.UseTargetMarg = p.BoolWithFallback(pUseTargetMarg, false)
}

func savePrefs(a fyne.App, c *Calc, alis, satis, kargo, komisyon, carpan, targetMargin *widget.Entry) {
	p := a.Preferences()
	p.SetString(pAlis, alis.Text)
	p.SetString(pSatis, satis.Text)
	p.SetString(pKargo, kargo.Text)
	p.SetString(pKomisyon, komisyon.Text)
	p.SetString(pCarpan, carpan.Text)
	p.SetString(pTargetMargin, targetMargin.Text)
	p.SetBool(pAuto, c.AutoFill)
	p.SetBool(pUseTargetMarg, c.UseTargetMarg)
}
