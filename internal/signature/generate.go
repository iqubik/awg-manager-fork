package signature

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultGenerateMTU = 1280
	maxSignatureSize   = 4096
)

type GenerateResult struct {
	OK       bool   `json:"ok"`
	Source   string `json:"source"`
	Protocol string `json:"protocol"`
	ByteSize int    `json:"byteSize"`
	Packets  struct {
		I1 string `json:"i1"`
		I2 string `json:"i2"`
		I3 string `json:"i3"`
		I4 string `json:"i4"`
		I5 string `json:"i5"`
	} `json:"packets"`
	Warning string `json:"warning,omitempty"`
}

type generatorInput struct {
	profile        string
	mtu            int
	useTagC        bool
	useTagT        bool
	useTagR        bool
	useTagRC       bool
	useTagRD       bool
	mimicAll       bool
	junkLevel      int
	iterCount      int
	routerMode     bool
	useExtremeMax  bool
	intensity      string
	useBrowserFp   bool
	browserProfile string
}

type generatorState struct {
	rnd *rand.Rand
}

type fpRange struct {
	min int
	max int
}

var protocolAliases = map[string]string{
	"quic_initial":     "quic_initial",
	"quic_0rtt":        "quic_0rtt",
	"tls":              "tls_client_hello",
	"tls_client_hello": "tls_client_hello",
	"wireguard_noise":  "wireguard_noise",
	"dtls":             "dtls",
	"http3":            "http3",
	"sip":              "sip",
	"dns_query":        "dns_query",
}

var hostPools = map[string][]string{
	"quic_initial":     {"yandex.net", "vk.com", "gcore.com", "github.com"},
	"quic_0rtt":        {"yandex.net", "ozon.ru", "spotify.com", "cloudfront.net"},
	"tls_client_hello": {"yandex.ru", "sber.ru", "github.com", "registry.npmjs.org"},
	"wireguard_noise":  {"noise.example"},
	"dtls":             {"stun.yandex.net", "meet.jit.si", "stun.services.mozilla.com"},
	"http3":            {"yandex.net", "vk.com", "gcore.com", "cloudfront.net"},
	"sip":              {"sip.mts.ru", "sip.zadarma.com", "sip.linphone.org"},
	"dns_query":        {"77.88.8.8", "8.8.8.8", "1.1.1.1", "9.9.9.9"},
}

var (
	cpsHexTagRe   = regexp.MustCompile(`<b 0x([0-9a-f]+)>`)
	cpsCountTagRe = regexp.MustCompile(`<(r|rc|rd) (\d+)>`)
)

func Generate(protocol string, mtu int) (GenerateResult, error) {
	resolved, ok := protocolAliases[strings.TrimSpace(protocol)]
	if !ok {
		return GenerateResult{}, fmt.Errorf("invalid protocol: %s", protocol)
	}
	if mtu <= 0 {
		mtu = defaultGenerateMTU
	}

	input := generatorInput{
		profile:        resolved,
		mtu:            mtu,
		useTagC:        false,
		useTagT:        true,
		useTagR:        true,
		useTagRC:       true,
		useTagRD:       true,
		mimicAll:       false,
		junkLevel:      5,
		iterCount:      0,
		routerMode:     false,
		useExtremeMax:  false,
		intensity:      "medium",
		useBrowserFp:   false,
		browserProfile: "",
	}
	state := &generatorState{rnd: rand.New(rand.NewSource(rand.Int63()))}

	iv := 2
	i1 := state.genI1(input, resolved, iv)
	i2 := state.mkEntropy(input, 1, iv)
	i3 := state.mkEntropy(input, 2, iv)
	i4 := state.mkEntropy(input, 3, iv)
	i5 := state.mkEntropy(input, 4, iv)
	if resolved == "dns_query" && input.mimicAll {
		i2 = state.mkDNS(input, iv+1)
		i3 = state.mkDNS(input, iv+2)
		i4 = state.mkDNS(input, iv+3)
		i5 = state.mkDNS(input, iv+4)
	}
	if input.routerMode {
		i2, i3, i4, i5 = "", "", "", ""
	}

	byteSize := CalcTotalSignatureByteSize(i1, i2, i3, i4, i5)
	if byteSize > maxSignatureSize {
		return GenerateResult{}, fmt.Errorf("signature too large: %d", byteSize)
	}

	var result GenerateResult
	result.OK = true
	result.Source = "generated"
	result.Protocol = resolved
	result.ByteSize = byteSize
	result.Packets.I1 = i1
	result.Packets.I2 = i2
	result.Packets.I3 = i3
	result.Packets.I4 = i4
	result.Packets.I5 = i5
	return result, nil
}

func SupportedGenerateProtocols() []string {
	return []string{
		"quic_initial",
		"quic_0rtt",
		"tls",
		"tls_client_hello",
		"wireguard_noise",
		"dtls",
		"http3",
		"sip",
		"dns_query",
	}
}

func CalcCPSByteSize(pattern string) int {
	size := 0

	for _, match := range cpsHexTagRe.FindAllStringSubmatch(pattern, -1) {
		size += len(match[1]) / 2
	}
	for _, match := range cpsCountTagRe.FindAllStringSubmatch(pattern, -1) {
		n, err := strconv.Atoi(match[2])
		if err == nil && n > 0 {
			size += n
		}
	}
	size += strings.Count(pattern, "<c>") * 4
	size += strings.Count(pattern, "<t>") * 4

	return size
}

func CalcTotalSignatureByteSize(packets ...string) int {
	total := 0
	for _, packet := range packets {
		total += CalcCPSByteSize(packet)
	}
	return total
}

func splitPad(n int, tag string) string {
	n = max(0, n)
	if n == 0 {
		return ""
	}
	var b strings.Builder
	for n > 1000 {
		b.WriteString(fmt.Sprintf("<%s 1000>", tag))
		n -= 1000
	}
	b.WriteString(fmt.Sprintf("<%s %d>", tag, n))
	return b.String()
}

func tagOverhead(useC, useT bool) int {
	out := 0
	if useC {
		out += 4
	}
	if useT {
		out += 4
	}
	return out
}

func calcPadding(headerB, extraB int, r *fpRange, iv, mtu int, s *generatorState) int {
	maxPad := max(0, mtu-headerB-extraB)
	if r == nil {
		return min(min(s.rndInt(20, 80)*iv, 500), maxPad)
	}
	occupied := headerB + extraB
	clampedMin := min(r.min, mtu)
	clampedMax := min(r.max, mtu)
	needed := max(0, clampedMin-occupied)
	jitter := max(0, min(min(clampedMax-clampedMin, clampedMax-occupied-needed), 20))
	pad := needed
	if jitter > 0 {
		pad += s.rndInt(0, jitter)
	}
	return min(pad, maxPad)
}

func getHost(input generatorInput, poolKey string, s *generatorState) string {
	actualKey := poolKey
	if poolKey == "dns_query" {
		actualKey = "dns_query"
	}
	pool := hostPools[actualKey]
	if len(pool) == 0 {
		pool = hostPools["tls_client_hello"]
	}
	return pool[s.rndInt(0, len(pool)-1)]
}

func getFpRange(input generatorInput, slot string) *fpRange {
	if !input.useBrowserFp || input.browserProfile == "" {
		return nil
	}

	switch slot {
	case "qi":
		return &fpRange{min: 1200, max: 1252}
	case "q0":
		return &fpRange{min: 1250, max: 1350}
	case "h3":
		return &fpRange{min: 1250, max: 1350}
	case "tls":
		return &fpRange{min: 512, max: 800}
	case "nx":
		return &fpRange{min: 1200, max: 1250}
	case "dtls":
		return &fpRange{min: 1050, max: 1200}
	default:
		return nil
	}
}

func alignTo128(n int) int {
	return int(math.Ceil(float64(n)/128.0) * 128.0)
}

func (s *generatorState) mkQUICi(input generatorInput, iv int) string {
	host := getHost(input, "quic_initial", s)
	dcid := s.rndInt(8, 20)
	scid := s.rndInt(0, 20)
	tokenLen := 0
	if s.rndInt(0, 1) == 1 {
		tokenLen = s.rndInt(8, 32)
	}
	sniRC := min(hostLen(host)+s.rndInt(0, 6), 64)
	hexPart := evenHex(
		hexPad(0xc0|s.rndInt(0, 3), 1) +
			"00000001" +
			hexPad(dcid, 1) +
			s.randHex(dcid) +
			hexPad(scid, 1) +
			s.randHex(scid) +
			hexPad(tokenLen, 1) +
			s.randHex(tokenLen) +
			s.randHex(4),
	)
	headerB := len(hexPart) / 2
	extraB := tagOverhead(input.useTagC, input.useTagT)
	if input.useTagRC {
		extraB += sniRC
	}
	pad := calcPadding(headerB, extraB, getFpRange(input, "qi"), iv, input.mtu, s)
	return "<b 0x" + hexPart + ">" +
		maybeRC(input.useTagRC, sniRC) +
		maybeTag(input.useTagC, "<c>") +
		maybeTag(input.useTagT, "<t>") +
		maybePad(input.useTagR, pad, "r")
}

func (s *generatorState) mkQUIC0(input generatorInput, iv int) string {
	host := getHost(input, "quic_0rtt", s)
	dcid := s.rndInt(8, 20)
	scid := s.rndInt(0, 20)
	ticketHint := min(hostLen(host)+s.rndInt(4, 16), 48)
	hexPart := evenHex(
		hexPad(0xd0|s.rndInt(0, 3), 1) +
			"00000001" +
			hexPad(dcid, 1) +
			s.randHex(dcid) +
			hexPad(scid, 1) +
			s.randHex(scid) +
			s.randHex(4),
	)
	headerB := len(hexPart) / 2
	extraB := tagOverhead(input.useTagC, input.useTagT)
	if input.useTagRC {
		extraB += ticketHint
	}
	pad := calcPadding(headerB, extraB, getFpRange(input, "q0"), iv, input.mtu, s)
	return "<b 0x" + hexPart + ">" +
		maybeTag(input.useTagT, "<t>") +
		maybePad(input.useTagR, pad, "r") +
		maybeRC(input.useTagRC, ticketHint) +
		maybeTag(input.useTagC, "<c>")
}

func (s *generatorState) mkTLS(input generatorInput, iv int) string {
	host := getHost(input, "tls_client_hello", s)
	sniExt := 2 + 2 + 2 + 1 + 2 + hostLen(host)
	sniRC := min(sniExt, 64)
	baseLen := s.rndInt(512, 800)
	recLen := alignTo128(baseLen)
	hsLen := recLen - s.rndInt(4, 9)
	rLen := min(min(s.rndInt(20, 60)*iv, 300), max(0, input.mtu-44-sniRC-tagOverhead(input.useTagC, input.useTagT)))
	hexPart := evenHex("160301" + hexPad(recLen, 2) + "01" + hexPad(hsLen, 3) + "0303" + s.randHex(32))
	return "<b 0x" + hexPart + ">" +
		maybeRC(input.useTagRC, sniRC) +
		maybePad(input.useTagR, rLen, "r") +
		maybeTag(input.useTagC, "<c>") +
		maybeTag(input.useTagT, "<t>")
}

func (s *generatorState) mkNoise(input generatorInput, iv int) string {
	rcLen := s.rndInt(4, 12)
	headerB := 148
	extraB := tagOverhead(input.useTagC, input.useTagT)
	if input.useTagRC {
		extraB += rcLen
	}
	pad := calcPadding(headerB, extraB, getFpRange(input, "nx"), iv, input.mtu, s)
	return "<b 0x01000000" + s.randHex(4) + ">" +
		"<b 0x" + s.randHex(32) + ">" +
		"<b 0x" + s.randHex(48) + ">" +
		"<b 0x" + s.randHex(28) + ">" +
		"<b 0x" + s.randHex(32) + ">" +
		maybePad(input.useTagR, pad, "r") +
		maybeTag(input.useTagT, "<t>") +
		maybeRC(input.useTagRC, rcLen)
}

func (s *generatorState) mkDTLS(input generatorInput, iv int) string {
	host := getHost(input, "dtls", s)
	fragLen := s.rndInt(100, 300)
	sniRC := min(hostLen(host)+s.rndInt(2, 8), 60)
	epoch := s.rndInt(0, 255)
	hexPart := evenHex(
		"16" + "fefd" + hexPad(epoch, 2) + s.randHex(6) + hexPad(fragLen, 2) + "01" + s.randHex(6) + "fefd0000" + s.randHex(4) + s.randHex(32),
	)
	headerB := len(hexPart) / 2
	extraB := tagOverhead(input.useTagC, input.useTagT)
	if input.useTagRC {
		extraB += sniRC
	}
	pad := calcPadding(headerB, extraB, getFpRange(input, "dtls"), iv, input.mtu, s)
	return "<b 0x" + hexPart + ">" +
		maybeRC(input.useTagRC, sniRC) +
		maybeTag(input.useTagC, "<c>") +
		maybeTag(input.useTagT, "<t>") +
		maybePad(input.useTagR, pad, "r")
}

func (s *generatorState) mkHTTP3(input generatorInput, iv int) string {
	host := getHost(input, "quic_initial", s)
	ptypes := []int{0xc0, 0xc1, 0xc2, 0xc3, 0xe0, 0xe1, 0xe2}
	dcid := s.rndInt(8, 20)
	scid := s.rndInt(0, 20)
	sniLen := min(hostLen(host)+9+s.rndInt(0, 6), 64)
	hexPart := evenHex(hexPad(ptypes[s.rndInt(0, len(ptypes)-1)], 1) + "00000001" + hexPad(dcid, 1) + s.randHex(dcid) + hexPad(scid, 1) + s.randHex(scid) + s.randHex(4))
	headerB := len(hexPart) / 2
	extraB := tagOverhead(input.useTagC, input.useTagT)
	if input.useTagRC {
		extraB += sniLen
	}
	pad := calcPadding(headerB, extraB, getFpRange(input, "h3"), iv, input.mtu, s)
	return "<b 0x" + hexPart + ">" +
		maybeRC(input.useTagRC, sniLen) +
		maybePad(input.useTagR, pad, "r") +
		maybeTag(input.useTagC, "<c>") +
		maybeTag(input.useTagT, "<t>")
}

func (s *generatorState) mkSIP(input generatorInput, iv int) string {
	host := getHost(input, "sip", s)
	hostHex := hex.EncodeToString([]byte(host))
	hexPart := evenHex("524547495354455220736970" + "3a" + hostHex + "20" + s.randHex(4))
	headerB := len(hexPart) / 2
	rcVal := min(hostLen(host)+s.rndInt(8, 24)*iv, 150)
	rLen := min(min(s.rndInt(5, 30)*iv, 120), max(0, input.mtu-headerB-rcVal-tagOverhead(input.useTagC, input.useTagT)))
	return "<b 0x" + hexPart + ">" +
		maybeRC(input.useTagRC, rcVal) +
		maybeTag(input.useTagC, "<c>") +
		maybeTag(input.useTagT, "<t>") +
		maybePad(input.useTagR, rLen, "r")
}

func (s *generatorState) mkDNS(input generatorInput, iv int) string {
	host := getHost(input, "dns_query", s)
	var queryName strings.Builder
	for _, label := range strings.Split(host, ".") {
		queryName.WriteString(fmt.Sprintf("%02x", len(label)))
		queryName.WriteString(hex.EncodeToString([]byte(label)))
	}
	queryName.WriteString("00")
	txid := s.randHex(2)
	qtype := "001c"
	if iv%2 == 0 {
		qtype = "0001"
	}
	hexPart := evenHex(txid + "0100" + "0001" + "0000" + "0000" + "0000" + queryName.String() + qtype + "0001")
	headerB := len(hexPart) / 2
	targetSize := s.rndInt(64, min(512, input.mtu-20))
	rLen := max(0, targetSize-headerB)
	return "<b 0x" + hexPart + ">" +
		maybePad(input.useTagR && rLen > 0, min(rLen, 200), "r") +
		maybeTag(input.useTagT, "<t>") +
		maybeTag(input.useTagC, "<c>")
}

func (s *generatorState) mkEntropy(input generatorInput, idx, iv int) string {
	isBig := s.rndInt(1, 10) > 6
	baseLen := s.rndInt(4, 20)
	limit := 60
	if isBig {
		baseLen = s.rndInt(200, 500)
		limit = 500
	}
	rLen := min(min(baseLen*iv, limit), max(0, input.mtu-20-tagOverhead(input.useTagC, input.useTagT)))
	rcLen := s.rndInt(4, 12)
	rdLen := s.rndInt(4, 8)
	c := maybeTag(input.useTagC, "<c>")
	t := maybeTag(input.useTagT, "<t>")
	r := maybePad(input.useTagR, rLen, "r")
	rc := maybeRC(input.useTagRC, rcLen)
	rd := maybePad(input.useTagRD, rdLen, "rd")
	b := ""
	if iv >= 2 {
		b = "<b 0x" + s.randHex(s.rndInt(4, 8*iv)) + ">"
	}
	b2 := ""
	if iv >= 3 {
		b2 = "<b 0x" + s.randHex(s.rndInt(2, 4)) + ">"
	}
	patterns := []string{
		b + r + t + rc + c + rd,
		c + t + b + r + rc + rd,
		rc + b + r + c + t + rd,
		t + r + c + rc + b + rd,
		r + rc + b + t + c + rd,
		b2 + t + r + b + rc + c + rd,
		rd + b + rc + r + t + c + b2,
		c + b + b2 + t + rc + r + rd,
	}
	result := patterns[(idx+s.rndInt(0, len(patterns)-1))%len(patterns)]
	if result == "" {
		return "<r 10>"
	}
	return result
}

func (s *generatorState) genI1(input generatorInput, profile string, iv int) string {
	switch profile {
	case "quic_initial":
		return s.mkQUICi(input, iv)
	case "quic_0rtt":
		return s.mkQUIC0(input, iv)
	case "tls_client_hello":
		return s.mkTLS(input, iv)
	case "wireguard_noise":
		return s.mkNoise(input, iv)
	case "dtls":
		return s.mkDTLS(input, iv)
	case "http3":
		return s.mkHTTP3(input, iv)
	case "sip":
		return s.mkSIP(input, iv)
	case "dns_query":
		return s.mkDNS(input, iv)
	default:
		return s.mkQUICi(input, iv)
	}
}

func (s *generatorState) rndInt(a, b int) int {
	if b <= a {
		return a
	}
	return s.rnd.Intn(b-a+1) + a
}

func (s *generatorState) randHex(n int) string {
	buf := make([]byte, max(0, n))
	for i := range buf {
		buf[i] = byte(s.rndInt(0, 255))
	}
	return hex.EncodeToString(buf)
}

func hexPad(value, byteLen int) string {
	hexValue := fmt.Sprintf("%x", value)
	for len(hexValue) < byteLen*2 {
		hexValue = "0" + hexValue
	}
	if len(hexValue) > byteLen*2 {
		return hexValue[len(hexValue)-byteLen*2:]
	}
	return hexValue
}

func evenHex(v string) string {
	if len(v)%2 != 0 {
		return v + "0"
	}
	return v
}

func maybeTag(enabled bool, tag string) string {
	if enabled {
		return tag
	}
	return ""
}

func maybeRC(enabled bool, n int) string {
	if enabled {
		return fmt.Sprintf("<rc %d>", n)
	}
	return ""
}

func maybePad(enabled bool, n int, tag string) string {
	if enabled && n > 0 {
		return splitPad(n, tag)
	}
	return ""
}

func hostLen(host string) int { return len(host) }
