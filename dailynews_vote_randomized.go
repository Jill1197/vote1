package main

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ==================== 2CAPTCHA CONFIG ====================
const (
	TwoCaptchaAPIKey  = "08aae54a0365bc997dd1978f4dcaa2fa" // ใส่ API key ตรงนี้
	TwoCaptchaEnabled = false                              // เปิด/ปิด 2Captcha
)

// ==================== 2CAPTCHA TURNSTILE ====================

type TwoCaptchaResponse struct {
	Status  int    `json:"status"`
	Request string `json:"request"`
}

// solveTurnstile ส่ง request ไป 2Captcha เพื่อ solve Cloudflare Turnstile
func solveTurnstile(siteKey, pageURL string) (string, error) {
	if !TwoCaptchaEnabled || TwoCaptchaAPIKey == "YOUR_2CAPTCHA_API_KEY" {
		return "", fmt.Errorf("2Captcha not configured")
	}

	// Step 1: ส่ง captcha ไป solve
	submitURL := fmt.Sprintf(
		"https://2captcha.com/in.php?key=%s&method=turnstile&sitekey=%s&pageurl=%s&json=1",
		TwoCaptchaAPIKey, siteKey, url.QueryEscape(pageURL),
	)

	resp, err := http.Get(submitURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var submitResp TwoCaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&submitResp); err != nil {
		return "", err
	}

	if submitResp.Status != 1 {
		return "", fmt.Errorf("2Captcha submit error: %s", submitResp.Request)
	}

	captchaID := submitResp.Request
	fmt.Printf("  [2Captcha] Submitted, ID: %s\n", captchaID)

	// Step 2: รอผล (poll ทุก 5 วินาที)
	for i := 0; i < 24; i++ { // timeout 2 นาที
		time.Sleep(5 * time.Second)

		resultURL := fmt.Sprintf(
			"https://2captcha.com/res.php?key=%s&action=get&id=%s&json=1",
			TwoCaptchaAPIKey, captchaID,
		)

		resp, err := http.Get(resultURL)
		if err != nil {
			continue
		}

		var resultResp TwoCaptchaResponse
		if err := json.NewDecoder(resp.Body).Decode(&resultResp); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		if resultResp.Status == 1 {
			fmt.Printf("  [2Captcha] Solved!\n")
			return resultResp.Request, nil
		}

		if resultResp.Request != "CAPCHA_NOT_READY" {
			return "", fmt.Errorf("2Captcha error: %s", resultResp.Request)
		}
	}

	return "", fmt.Errorf("2Captcha timeout")
}

// findTurnstileSiteKey หา sitekey จาก HTML
func findTurnstileSiteKey(html string) string {
	// Pattern: data-sitekey="xxx" หรือ sitekey: 'xxx'
	patterns := []string{
		`data-sitekey="`,
		`sitekey":"`,
		`sitekey: '`,
		`sitekey:'`,
	}

	for _, pattern := range patterns {
		if idx := strings.Index(html, pattern); idx != -1 {
			start := idx + len(pattern)
			end := strings.IndexAny(html[start:], `"'`)
			if end != -1 && end < 100 {
				return html[start : start+end]
			}
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ==================== RANDOM USER AGENTS ====================
var UserAgents = []string{
	// Chrome Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 11.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 11.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",

	// Chrome Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",

	// Firefox Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:132.0) Gecko/20100101 Firefox/132.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:131.0) Gecko/20100101 Firefox/131.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:129.0) Gecko/20100101 Firefox/129.0",

	// Firefox Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:133.0) Gecko/20100101 Firefox/133.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14.0; rv:133.0) Gecko/20100101 Firefox/133.0",

	// Safari Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",

	// Edge Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36 Edg/129.0.0.0",

	// Chrome Linux
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",

	// Firefox Linux
	"Mozilla/5.0 (X11; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0",

	// Mobile Chrome Android
	"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; SAMSUNG SM-A546B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36",

	// Mobile Safari iOS
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",

	// Opera
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 OPR/116.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 OPR/115.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 OPR/116.0.0.0",
}

// sec-ch-ua ที่ match กับ User-Agent
var SecChUaMap = map[string]string{
	"Chrome/131": `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
	"Chrome/130": `"Google Chrome";v="130", "Chromium";v="130", "Not_A Brand";v="24"`,
	"Chrome/129": `"Google Chrome";v="129", "Chromium";v="129", "Not_A Brand";v="24"`,
	"Chrome/128": `"Google Chrome";v="128", "Chromium";v="128", "Not_A Brand";v="24"`,
	"Chrome/127": `"Google Chrome";v="127", "Chromium";v="127", "Not_A Brand";v="24"`,
	"Chrome/126": `"Google Chrome";v="126", "Chromium";v="126", "Not_A Brand";v="24"`,
	"Chrome/125": `"Google Chrome";v="125", "Chromium";v="125", "Not_A Brand";v="24"`,
	"Edge/131":   `"Microsoft Edge";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
	"Edge/130":   `"Microsoft Edge";v="130", "Chromium";v="130", "Not_A Brand";v="24"`,
	"Edge/129":   `"Microsoft Edge";v="129", "Chromium";v="129", "Not_A Brand";v="24"`,
	"OPR/116":    `"Opera";v="116", "Chromium";v="131", "Not_A Brand";v="24"`,
	"OPR/115":    `"Opera";v="115", "Chromium";v="130", "Not_A Brand";v="24"`,
}

// ==================== RANDOM VOTER DATA ====================
var Genders = []string{"ชาย", "หญิง"}
var Ages = []string{"18-28ปี", "29-44ปี", "45-59ปี", "60ปีขึ้นไป"}
var Statuses = []string{"โสด", "สมรส", "หม้าย/หย่า"}
var Educations = []string{"ประถม", "มัธยม", "ปวช./ปวส.", "ปริญญาตรี", "สูงกว่าปริญญาตรี"}
var Jobs = []string{"นักเรียน/นักศึกษา", "ข้าราชการ", "พนักงานเอกชน", "ธุรกิจส่วนตัว", "เกษตรกร", "รับจ้างทั่วไป", "ว่างงาน", "อื่นๆ"}
var Regions = []string{
	"กรุงเทพมหานคร", "กระบี่", "กาญจนบุรี", "กาฬสินธุ์", "กำแพงเพชร",
	"ขอนแก่น", "จันทบุรี", "ฉะเชิงเทรา", "ชลบุรี", "ชัยนาท",
	"ชัยภูมิ", "ชุมพร", "เชียงราย", "เชียงใหม่", "ตรัง",
	"ตราด", "ตาก", "นครนายก", "นครปฐม", "นครพนม",
	"นครราชสีมา", "นครศรีธรรมราช", "นครสวรรค์", "นนทบุรี", "นราธิวาส",
	"น่าน", "บึงกาฬ", "บุรีรัมย์", "ปทุมธานี", "ประจวบคีรีขันธ์",
	"ปราจีนบุรี", "ปัตตานี", "พระนครศรีอยุธยา", "พังงา", "พัทลุง",
	"พิจิตร", "พิษณุโลก", "เพชรบุรี", "เพชรบูรณ์", "แพร่",
	"พะเยา", "ภูเก็ต", "มหาสารคาม", "มุกดาหาร", "แม่ฮ่องสอน",
	"ยะลา", "ยโสธร", "ร้อยเอ็ด", "ระนอง", "ระยอง",
	"ราชบุรี", "ลพบุรี", "ลำปาง", "ลำพูน", "เลย",
	"ศรีสะเกษ", "สกลนคร", "สงขลา", "สตูล", "สมุทรปราการ",
	"สมุทรสงคราม", "สมุทรสาคร", "สระแก้ว", "สระบุรี", "สิงห์บุรี",
	"สุโขทัย", "สุพรรณบุรี", "สุราษฎร์ธานี", "สุรินทร์", "หนองคาย",
	"หนองบัวลำภู", "อ่างทอง", "อุดรธานี", "อุทัยธานี", "อุตรดิตถ์",
	"อุบลราชธานี", "อำนาจเจริญ",
}
var Incomes = []string{"ต่ำกว่า10,000บาท", "10,001-20,000บาท", "20,001-30,000บาท", "30,001-50,000บาท", "50,001บาทขึ้นไป"}

// ==================== HELPER FUNCTIONS ====================

func randomChoice(choices []string) string {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(choices))))
	return choices[n.Int64()]
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func generateRandomHex(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateBase36(length int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[n.Int64()]
	}
	return string(result)
}

func randomInt(min, max int64) int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(max-min))
	return min + n.Int64()
}

func generateGoogleAdsSignature() string {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, 22)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[n.Int64()]
	}
	return "ALNI_M" + string(result)
}

func generateCriteoBundle() string {
	data := make([]byte, 80)
	rand.Read(data)
	encoded := base64.StdEncoding.EncodeToString(data)
	return url.QueryEscape(encoded)
}

func generateFCCDCF(timestamp int64) string {
	consentID := generateUUID()
	inner := fmt.Sprintf(`[null,null,null,null,null,null,[[32,"[\"%s\",[%d,791000000]]"]]]`, consentID, timestamp)
	return url.QueryEscape(inner)
}

// ==================== BROWSER FINGERPRINT ====================

type BrowserFingerprint struct {
	UserAgent       string
	SecChUa         string
	SecChUaMobile   string
	SecChUaPlatform string
	AcceptLanguage  string
}

func GenerateRandomFingerprint() BrowserFingerprint {
	ua := randomChoice(UserAgents)

	// หา sec-ch-ua ที่ match
	secChUa := `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"` // default
	for key, val := range SecChUaMap {
		if strings.Contains(ua, key) {
			secChUa = val
			break
		}
	}

	// หา platform
	platform := `"Windows"`
	if strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS") {
		platform = `"macOS"`
	} else if strings.Contains(ua, "Linux") && !strings.Contains(ua, "Android") {
		platform = `"Linux"`
	} else if strings.Contains(ua, "Android") {
		platform = `"Android"`
	} else if strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") {
		platform = `"iOS"`
	}

	// Mobile?
	mobile := "?0"
	if strings.Contains(ua, "Mobile") || strings.Contains(ua, "Android") || strings.Contains(ua, "iPhone") {
		mobile = "?1"
	}

	// Random Accept-Language
	languages := []string{
		"th-TH,th;q=0.9,en-US;q=0.8,en;q=0.7",
		"th,en-US;q=0.9,en;q=0.8",
		"th-TH,th;q=0.9,en;q=0.8",
		"en-US,en;q=0.9,th;q=0.8",
		"th-TH,th;q=0.8,en-US;q=0.6,en;q=0.4",
		"th;q=0.9,en-US;q=0.8,en;q=0.7",
	}

	return BrowserFingerprint{
		UserAgent:       ua,
		SecChUa:         secChUa,
		SecChUaMobile:   mobile,
		SecChUaPlatform: platform,
		AcceptLanguage:  randomChoice(languages),
	}
}

// ==================== COOKIES ====================

type DailynewsCookies struct {
	LotameCID, LotameSID, LotameDomainCheck      string
	CBClose, CBClose61341, UID61341, CTout61341  string
	GA, GID, GA4Session, GAT1, GAT7              string
	ClarityClick, ClaritySession                 string
	Gads, Gpi, Eoi                               string
	CtoBundle                                    string
	PanoramaID, PanoramaIDType, PanoramaIDExpiry string
	FCCDCF, CCID                                 string
}

func GenerateDailynewsCookies() DailynewsCookies {
	now := time.Now()
	timestamp := now.Unix()
	timestampMs := now.UnixMilli()
	expiryMs := now.Add(7 * 24 * time.Hour).UnixMilli()

	return DailynewsCookies{
		LotameCID:         generateUUID(),
		LotameSID:         fmt.Sprintf("%s-%s", generateRandomHex(4), generateRandomHex(4)),
		LotameDomainCheck: "dailynews.co.th",
		CBClose:           "1",
		CBClose61341:      "1",
		UID61341:          fmt.Sprintf("%s.%d", strings.ToUpper(generateRandomHex(4)), randomInt(1, 5)),
		CTout61341:        "1",
		GA:                fmt.Sprintf("GA1.3.%d.%d", randomInt(100000000, 999999999), timestamp),
		GID:               fmt.Sprintf("GA1.3.%d.%d", randomInt(1000000000, 1999999999), timestamp),
		GA4Session:        fmt.Sprintf("GS2.1.s%d$o1$g0$t%d$j%d$l0$h0", timestamp, timestamp, randomInt(30, 120)),
		GAT1:              "1",
		GAT7:              "1",
		ClarityClick:      fmt.Sprintf("%s%%5E2%%5Eg25%%5E0%%5E%d", generateBase36(7), randomInt(1000, 9999)),
		ClaritySession:    fmt.Sprintf("%s%%5E%d%%5E1%%5E1%%5Ej.clarity.ms%%2Fcollect", generateBase36(6), timestampMs),
		Gads:              fmt.Sprintf("ID=%s:T=%d:RT=%d:S=%s", generateRandomHex(8), timestamp, timestamp, generateGoogleAdsSignature()),
		Gpi:               fmt.Sprintf("UID=%s:T=%d:RT=%d:S=%s", generateRandomHex(8), timestamp, timestamp, generateGoogleAdsSignature()),
		Eoi:               fmt.Sprintf("ID=%s:T=%d:RT=%d:S=%s", generateRandomHex(8), timestamp, timestamp, "AA-Afj"+generateBase36(20)),
		CtoBundle:         generateCriteoBundle(),
		PanoramaID:        generateRandomHex(32),
		PanoramaIDType:    "panoDevice",
		PanoramaIDExpiry:  fmt.Sprintf("%d", expiryMs),
		FCCDCF:            generateFCCDCF(timestamp),
		CCID:              generateRandomHex(16),
	}
}

func (c DailynewsCookies) ToCookieString() string {
	parts := []string{
		fmt.Sprintf("__lt__cid=%s", c.LotameCID),
		fmt.Sprintf("__lt__sid=%s", c.LotameSID),
		fmt.Sprintf("_cbclose=%s", c.CBClose),
		fmt.Sprintf("_cbclose61341=%s", c.CBClose61341),
		fmt.Sprintf("_uid61341=%s", c.UID61341),
		fmt.Sprintf("_ctout61341=%s", c.CTout61341),
		fmt.Sprintf("_ga_J6Z20LRSP9=%s", c.GA4Session),
		fmt.Sprintf("_ga=%s", c.GA),
		fmt.Sprintf("_gid=%s", c.GID),
		fmt.Sprintf("_gat_gtag_UA_27011568_1=%s", c.GAT1),
		fmt.Sprintf("_gat_gtag_UA_27011568_7=%s", c.GAT7),
		fmt.Sprintf("_clck=%s", c.ClarityClick),
		fmt.Sprintf("FCCDCF=%s", c.FCCDCF),
		fmt.Sprintf("lotame_domain_check=%s", c.LotameDomainCheck),
		fmt.Sprintf("_cc_id=%s", c.CCID),
		fmt.Sprintf("panoramaId_expiry=%s", c.PanoramaIDExpiry),
		fmt.Sprintf("panoramaId=%s", c.PanoramaID),
		fmt.Sprintf("panoramaIdType=%s", c.PanoramaIDType),
		fmt.Sprintf("cto_bundle=%s", c.CtoBundle),
		fmt.Sprintf("__gads=%s", c.Gads),
		fmt.Sprintf("__gpi=%s", c.Gpi),
		fmt.Sprintf("__eoi=%s", c.Eoi),
		fmt.Sprintf("_clsk=%s", c.ClaritySession),
	}
	return strings.Join(parts, "; ")
}

// ==================== VOTER INFO ====================

type VoterInfo struct {
	Gender, Age, Status, Education, Job, Region, Income string
}

func GenerateRandomVoter() VoterInfo {
	return VoterInfo{
		Gender:    randomChoice(Genders),
		Age:       randomChoice(Ages),
		Status:    randomChoice(Statuses),
		Education: randomChoice(Educations),
		Job:       randomChoice(Jobs),
		Region:    randomChoice(Regions),
		Income:    randomChoice(Incomes),
	}
}

// ==================== FORM TOKENS ====================

type FormTokens struct {
	DataToken, FormID, FormKey, FrmSubmitEntry, FrmState, Nonce string
}

func ParseFormTokens(html string) FormTokens {
	tokens := FormTokens{}

	if idx := strings.Index(html, `data-token="`); idx != -1 {
		start := idx + len(`data-token="`)
		end := strings.Index(html[start:], `"`)
		if end != -1 && end < 50 {
			tokens.DataToken = html[start : start+end]
		}
	}

	if idx := strings.Index(html, `name="form_id" value="`); idx != -1 {
		start := idx + len(`name="form_id" value="`)
		end := strings.Index(html[start:], `"`)
		if end != -1 && end < 20 {
			tokens.FormID = html[start : start+end]
		}
	}

	if idx := strings.Index(html, `name="form_key" value="`); idx != -1 {
		start := idx + len(`name="form_key" value="`)
		end := strings.Index(html[start:], `"`)
		if end != -1 && end < 100 {
			tokens.FormKey = html[start : start+end]
		}
	}

	if idx := strings.Index(html, `name="frm_submit_entry_`); idx != -1 {
		sub := html[idx:]
		if vIdx := strings.Index(sub, `value="`); vIdx != -1 && vIdx < 50 {
			start := vIdx + len(`value="`)
			end := strings.Index(sub[start:], `"`)
			if end != -1 && end < 20 {
				tokens.FrmSubmitEntry = sub[start : start+end]
			}
		}
	}

	if idx := strings.Index(html, `name="frm_state"`); idx != -1 {
		sub := html[idx:]
		if vIdx := strings.Index(sub, `value="`); vIdx != -1 && vIdx < 50 {
			start := vIdx + len(`value="`)
			end := strings.Index(sub[start:], `"`)
			if end != -1 && end < 100 {
				tokens.FrmState = sub[start : start+end]
			}
		}
	}

	if idx := strings.Index(html, `var frm_js`); idx != -1 {
		sub := html[idx:]
		if len(sub) > 2000 {
			sub = sub[:2000]
		}
		if nIdx := strings.Index(sub, `"nonce": "`); nIdx != -1 {
			start := nIdx + len(`"nonce": "`)
			end := strings.Index(sub[start:], `"`)
			if end != -1 && end < 20 {
				tokens.Nonce = sub[start : start+end]
			}
		}
		if tokens.Nonce == "" {
			if nIdx := strings.Index(sub, `"nonce":"`); nIdx != -1 {
				start := nIdx + len(`"nonce":"`)
				end := strings.Index(sub[start:], `"`)
				if end != -1 && end < 20 {
					tokens.Nonce = sub[start : start+end]
				}
			}
		}
	}

	return tokens
}

// ==================== HTTP CLIENT WITH PROXY ====================

func createHTTPClient() *http.Client {
	transport := &http.Transport{}

	// อ่าน proxy จาก environment variable
	if proxyURL := os.Getenv("HTTP_PROXY"); proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxy)
		}
	} else if proxyURL := os.Getenv("HTTPS_PROXY"); proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxy)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// ==================== HTTP REQUESTS ====================

func FetchPollPage(cookies DailynewsCookies, fp BrowserFingerprint) (string, error) {
	client := createHTTPClient()

	req, err := http.NewRequest("GET", "https://www.dailynews.co.th/polls/election-2026/", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept-Language", fp.AcceptLanguage)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("User-Agent", fp.UserAgent)

	// Chrome-based browsers only
	if strings.Contains(fp.UserAgent, "Chrome") || strings.Contains(fp.UserAgent, "Edg") || strings.Contains(fp.UserAgent, "OPR") {
		req.Header.Set("sec-ch-ua", fp.SecChUa)
		req.Header.Set("sec-ch-ua-mobile", fp.SecChUaMobile)
		req.Header.Set("sec-ch-ua-platform", fp.SecChUaPlatform)
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-User", "?1")
	}

	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cookie", cookies.ToCookieString())

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	body, err := io.ReadAll(reader)
	return string(body), err
}

func SubmitVote(cookies DailynewsCookies, fp BrowserFingerprint, tokens FormTokens, partyName, pmName string, voter VoterInfo) (bool, string) {
	client := createHTTPClient()
	boundary := "----WebKitFormBoundary" + generateRandomHex(8)

	var body strings.Builder
	addField := func(name, value string) {
		body.WriteString("--" + boundary + "\r\n")
		body.WriteString(fmt.Sprintf(`Content-Disposition: form-data; name="%s"`, name) + "\r\n\r\n")
		body.WriteString(value + "\r\n")
	}

	addField("frm_action", "create")
	addField("form_id", tokens.FormID)
	addField("frm_hide_fields_"+tokens.FormID, "")
	addField("form_key", tokens.FormKey)
	addField("item_meta[0]", "")
	addField("frm_submit_entry_"+tokens.FormID, tokens.FrmSubmitEntry)
	addField("_wp_http_referer", "/polls/election-2026/")
	addField("item_meta[205]", partyName)
	addField("item_meta[206]", "")
	addField("item_meta[211]", pmName)
	addField("frm_next_page", "")
	addField("item_meta[217]", voter.Gender)
	addField("item_meta[218]", voter.Age)
	addField("item_meta[222]", voter.Status)
	addField("item_meta[219]", voter.Education)
	addField("item_meta[220]", voter.Job)
	addField("item_meta[216]", voter.Region)
	addField("item_meta[221]", voter.Income)
	addField("item_key", "")
	addField("frm_verify", "")
	addField("frm_state", tokens.FrmState)
	addField("antispam_token", tokens.DataToken)
	body.WriteString("--" + boundary + "--\r\n")

	req, err := http.NewRequest("POST", "https://www.dailynews.co.th/polls/election-2026/", strings.NewReader(body.String()))
	if err != nil {
		return false, err.Error()
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept-Language", fp.AcceptLanguage)
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.Header.Set("Origin", "https://www.dailynews.co.th")
	req.Header.Set("Referer", "https://www.dailynews.co.th/polls/election-2026/")
	req.Header.Set("User-Agent", fp.UserAgent)

	if strings.Contains(fp.UserAgent, "Chrome") || strings.Contains(fp.UserAgent, "Edg") || strings.Contains(fp.UserAgent, "OPR") {
		req.Header.Set("sec-ch-ua", fp.SecChUa)
		req.Header.Set("sec-ch-ua-mobile", fp.SecChUaMobile)
		req.Header.Set("sec-ch-ua-platform", fp.SecChUaPlatform)
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-User", "?1")
	}

	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cookie", "verify=test; "+cookies.ToCookieString())

	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return false, err.Error()
		}
		defer gzReader.Close()
		reader = gzReader
	}

	respBody, _ := io.ReadAll(reader)
	respStr := string(respBody)

	if strings.Contains(respStr, "Complete") || strings.Contains(respStr, "โหวตเรียบร้อยแล้ว") {
		return true, "SUCCESS"
	}

	return false, "FAILED"
}

// ==================== MAIN ====================

func main() {
	fmt.Println("=== Dailynews Poll Vote Bot (Randomized) ===\n")

	// สร้าง fingerprint สุ่ม
	fp := GenerateRandomFingerprint()
	fmt.Printf("[Fingerprint]\n")
	fmt.Printf("  User-Agent: %s\n", truncate(fp.UserAgent, 60)+"...")
	fmt.Printf("  Platform: %s\n", fp.SecChUaPlatform)
	fmt.Printf("  Mobile: %s\n", fp.SecChUaMobile)

	// สร้าง cookies สุ่ม
	cookies := GenerateDailynewsCookies()
	fmt.Printf("\n[Cookies]\n")
	fmt.Printf("  _ga: %s\n", cookies.GA)
	fmt.Printf("  __lt__cid: %s\n", cookies.LotameCID)

	// สร้างข้อมูลผู้โหวตสุ่ม
	voter := GenerateRandomVoter()
	fmt.Printf("\n[Voter]\n")
	fmt.Printf("  Gender: %s, Age: %s\n", voter.Gender, voter.Age)
	fmt.Printf("  Status: %s, Education: %s\n", voter.Status, voter.Education)
	fmt.Printf("  Job: %s, Region: %s\n", voter.Job, voter.Region)
	fmt.Printf("  Income: %s\n", voter.Income)

	// Fetch page
	fmt.Println("\n[Step 1] Fetching page...")
	html, err := FetchPollPage(cookies, fp)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
		return
	}
	fmt.Printf("  Got %d bytes\n", len(html))

	// เช็คว่าโดน Cloudflare block หรือไม่
	if strings.Contains(html, "Just a moment") || strings.Contains(html, "cf-turnstile") {
		fmt.Println("  [!] Cloudflare challenge detected!")

		// หา sitekey
		siteKey := findTurnstileSiteKey(html)
		if siteKey != "" && TwoCaptchaEnabled {
			fmt.Printf("  [2Captcha] Found sitekey: %s\n", siteKey)
			token, err := solveTurnstile(siteKey, "https://www.dailynews.co.th/polls/election-2026/")
			if err != nil {
				fmt.Printf("  [2Captcha] ERROR: %v\n", err)
				return
			}
			// TODO: ใช้ token submit ผ่าน Cloudflare
			fmt.Printf("  [2Captcha] Token: %s\n", truncate(token, 50))
		} else {
			fmt.Println("  ERROR: Cloudflare blocked, 2Captcha not enabled")
			return
		}
	}

	// Parse tokens
	tokens := ParseFormTokens(html)
	if tokens.DataToken == "" || tokens.FrmState == "" {
		fmt.Printf("  ERROR: Cannot parse tokens\n")
		return
	}
	fmt.Printf("  DataToken: %s\n", tokens.DataToken)
	fmt.Printf("  FrmState: %s\n", tokens.FrmState)

	// Submit vote
	partyName := "พรรคเพื่อไทย"
	pmName := "นายยศชนันวงศ์สวัสดิ์"

	fmt.Printf("\n[Step 2] Submitting vote...\n")
	fmt.Printf("  Party: %s\n", partyName)
	fmt.Printf("  PM: %s\n", pmName)

	success, msg := SubmitVote(cookies, fp, tokens, partyName, pmName, voter)
	if success {
		fmt.Printf("\n✓ VOTE SUCCESS!\n")
	} else {
		fmt.Printf("\n✗ VOTE FAILED: %s\n", msg)
	}
}
