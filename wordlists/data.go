package wordlists

import "fmt"

// Symbols commonly used in passwords
var Symbols = []string{
	"!", "@", "#", "$", "%", "^", "&", "*",
	"~", "`", "+", "=", "?", "/", "\\", "|",
	"(", ")", "[", "]", "{", "}", "<", ">",
	".", ",", ";", ":", "'", "\"",
	"!!", "@@", "##", "$$", "%%", "**",
	"!@", "@!", "#!", "!#", "$!", "!$",
	"!@#", "#@!", "!@#$", "$#@!", "!@#$%",
	"!@#$%^", "!@#$%^&", "!@#$%^&*",
	"*!", "!*", "@#", "#$", "$%", "%^", "^&", "&*",
	"!1", "!2", "@1", "@2", "#1", "#2",
	"!!", "!?", "?!", "!@!", "@!@", "#!#",
	"!#$", "$#!", "!$!", "$!$",
	"...", "---", "___", "***", "###",
}

// SymbolsShort is a smaller set for combinations to avoid explosion
var SymbolsShort = []string{
	"!", "@", "#", "$", "*", "%", "^", "&",
	"!!", "!@#", "!@", "@!", "#!",
}

// SymbolsFull is the full set for direct append/prepend
var SymbolsFull = []string{
	"!", "@", "#", "$", "%", "^", "&", "*", "~",
	"!!", "@@", "##", "$$", "**",
	"!@", "@!", "#!", "!#", "$!",
	"!@#", "#@!", "!@#$", "$#@!",
	"!@#$%", "!@#$%^", "!@#$%^&", "!@#$%^&*",
	"!1", "@2", "#3", "$4", "%5",
	"!?", "?!", "...", "---", "___",
	"!!!", "@@@", "###", "$$$",
}

// Digits 0-9
var Digits []string

// TwoDigits 00-99
var TwoDigits []string

// ThreeDigits 000-999
var ThreeDigits []string

// FourDigits - common 4-digit PINs
var FourDigits []string

// Years commonly used in passwords
var Years []string

// CommonSuffixWords are words commonly appended to passwords
var CommonSuffixWords = []string{
	// Generic
	"admin", "password", "pass", "passwd", "secure", "security",
	"corp", "inc", "ltd", "llc", "org", "com", "net", "io",
	"login", "welcome", "access", "master", "root", "user",
	"test", "temp", "backup", "sys", "system", "network",
	"web", "app", "server", "db", "data", "cloud",
	"dev", "prod", "staging", "internal", "external",
	"the", "my", "our", "is", "was",
	"wifi", "laptop", "desktop", "mobile", "phone",
	// Tech
	"tech", "cyber", "hack", "code", "linux", "windows", "mac",
	"azure", "aws", "gcp", "docker", "kube", "k8s",
	"api", "ssh", "vpn", "dns", "ssl", "tls", "http", "https",
	"git", "repo", "deploy", "build", "release", "debug",
	"proxy", "firewall", "gateway", "router", "switch",
	"node", "cluster", "host", "vm", "container",
	"sql", "mongo", "redis", "postgres", "mysql", "oracle",
	"token", "key", "secret", "cert", "auth", "oauth",
	"encrypt", "decrypt", "hash", "salt", "crypt",
	// Departments / roles
	"it", "hr", "eng", "ops", "devops", "sre", "infra",
	"cto", "ceo", "cfo", "ciso", "soc",
	"finance", "sales", "marketing", "support", "helpdesk",
	// Seasons / months
	"spring", "summer", "fall", "autumn", "winter",
	"jan", "feb", "mar", "apr", "may", "jun",
	"jul", "aug", "sep", "oct", "nov", "dec",
	"january", "february", "march", "april", "june",
	"july", "august", "september", "october", "november", "december",
	// Common password words
	"love", "life", "rock", "star", "king", "queen",
	"dragon", "monkey", "tiger", "lion", "bear", "wolf", "eagle",
	"fire", "ice", "thunder", "storm", "shadow", "dark", "light",
	"ninja", "pirate", "hero", "legend", "power", "force",
	"magic", "wizard", "angel", "demon", "god", "devil",
	"gold", "silver", "diamond", "platinum", "ruby",
	"one", "two", "three", "four", "five",
	"red", "blue", "green", "black", "white",
	"pro", "max", "ultra", "plus", "prime", "elite",
	"x", "xx", "xxx", "xo", "xoxo",
}

// CommonPrefixWords are words commonly prepended to passwords
var CommonPrefixWords = []string{
	"the", "my", "i", "we", "our", "a", "an",
	"admin", "super", "mega", "ultra", "best", "top",
	"team", "go", "love", "ilove", "ilov", "iloveu",
	"mr", "ms", "mrs", "dr", "sir",
	"big", "new", "old", "hot", "cool",
	"cyber", "tech", "hack", "code", "dev",
	"pro", "max", "hyper", "giga", "tera",
	"dark", "shadow", "black", "red", "blue",
	"king", "queen", "lord", "master", "chief",
	"secret", "hidden", "private", "public",
	"its", "this", "that",
}

// Separators placed between keyword and appended/prepended content
var Separators = []string{
	"", "_", "-", ".", "@", "#", "!", " ",
	"__", "--", "..", "@@", "##",
}

// SeparatorsShort for combination stages
var SeparatorsShort = []string{
	"", "_", "-", ".", "@", "#",
}

// KeyboardWalks common keyboard walk patterns
var KeyboardWalks = []string{
	"qwerty", "qwert", "qwer", "qwe",
	"asdf", "asdfg", "asdfgh",
	"zxcv", "zxcvb", "zxcvbn",
	"1qaz", "2wsx", "3edc", "4rfv",
	"qazwsx", "1qaz2wsx",
	"!QAZ", "!qaz", "@WSX", "@wsx",
	"1q2w3e", "1q2w3e4r",
	"zaq1", "xsw2", "cde3",
	"plm", "okn", "ijn", "uhb",
	"qwertyuiop", "asdfghjkl", "zxcvbnm",
}

// NumberSequences common number sequences
var NumberSequences = []string{
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	"00", "01", "10", "11", "12", "13", "21", "22", "23", "33", "42", "44",
	"55", "66", "69", "77", "88", "99",
	"000", "001", "007", "010", "069", "100", "111", "112", "123",
	"200", "222", "234", "300", "321", "333", "345",
	"400", "420", "444", "456", "500", "555", "567",
	"600", "666", "678", "700", "777", "786", "789",
	"800", "888", "900", "911", "999",
	"0000", "1111", "1234", "1337", "2222", "3333", "4321", "4444",
	"5555", "6666", "6969", "7777", "8888", "9999",
	"12345", "54321", "11111", "12321", "13579",
	"123456", "654321", "111111", "123321",
	"1234567", "12345678", "123456789", "1234567890",
	"0987654321", "9876543210",
	"1122", "2233", "3344", "1212", "2323", "1010",
	"2468", "1357", "0101", "1001", "2002", "3003",
}

// CommonPatterns are full patterns appended/prepended
var CommonPatterns []string

func init() {
	// Generate digits
	for i := 0; i <= 9; i++ {
		Digits = append(Digits, fmt.Sprintf("%d", i))
	}

	// Generate two-digit numbers
	for i := 0; i <= 99; i++ {
		TwoDigits = append(TwoDigits, fmt.Sprintf("%02d", i))
	}

	// Generate three-digit numbers
	for i := 0; i <= 999; i++ {
		ThreeDigits = append(ThreeDigits, fmt.Sprintf("%03d", i))
	}

	// Generate all four-digit numbers 0000-9999
	for i := 0; i <= 9999; i++ {
		FourDigits = append(FourDigits, fmt.Sprintf("%04d", i))
	}

	// Generate years
	for y := 1950; y <= 2050; y++ {
		Years = append(Years, fmt.Sprintf("%d", y))
	}
	// Short years 00-50, 70-99
	for y := 0; y <= 50; y++ {
		Years = append(Years, fmt.Sprintf("%02d", y))
	}
	for y := 70; y <= 99; y++ {
		Years = append(Years, fmt.Sprintf("%02d", y))
	}

	// Build common patterns: symbol + number combos
	patternSyms := []string{"!", "@", "#", "$", "%", "*", "!!", "!@#", "!@", "@!"}
	for _, s := range patternSyms {
		for _, y := range Years {
			CommonPatterns = append(CommonPatterns, y+s)
			CommonPatterns = append(CommonPatterns, s+y)
		}
	}
	for _, s := range patternSyms {
		for _, d := range Digits {
			CommonPatterns = append(CommonPatterns, d+s)
			CommonPatterns = append(CommonPatterns, s+d)
		}
	}
	// symbol + number + symbol patterns
	syms2 := []string{"!", "@", "#", "$", "%", "*"}
	for _, s1 := range syms2 {
		for _, s2 := range syms2 {
			for _, d := range []string{
				"1", "2", "3", "12", "13", "21", "23",
				"123", "321", "1234", "4321",
				"2027", "2026", "2025", "2024", "2023",
				"27", "26", "25", "24", "23",
			} {
				CommonPatterns = append(CommonPatterns, s1+d+s2)
			}
		}
	}
	// Multi-symbol + year
	multiSyms := []string{"!@#", "!@#$", "#@!", "$#@!", "!@#$%"}
	for _, s := range multiSyms {
		for _, y := range []string{"2024", "2025", "2026", "2027", "24", "25", "26", "27"} {
			CommonPatterns = append(CommonPatterns, s+y)
			CommonPatterns = append(CommonPatterns, y+s)
		}
	}
}
