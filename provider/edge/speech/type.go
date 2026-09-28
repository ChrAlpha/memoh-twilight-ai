package speech

import (
	_ "embed"
	"encoding/json"
	"slices"
)

const (
	edgeSpeechURL        = "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1"
	edgeAPIToken         = "6A5AA1D4EAFF4E9FB37E23D68491D6F4" //nolint:gosec // Well-known public token shared by all Edge TTS clients, not a secret.
	chromiumFullVersion  = "143.0.3650.75"
	chromiumMajorVersion = "143"

	wssOrigin = "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold"

	DefaultVoice = "en-US-EmmaMultilingualNeural"

	winEpochOffset = 11644473600
	sToNS          = 1000000000
)

//go:embed voices.json
var voicesJSON []byte

// voicesByLang maps language tags to voice IDs. It is populated once in init
// and never mutated afterwards, so concurrent reads are safe. It must not be
// handed out directly; use Voices, which returns a copy.
var voicesByLang map[string][]string

var voiceToLang map[string]string

func init() {
	if err := json.Unmarshal(voicesJSON, &voicesByLang); err != nil {
		panic("edge: failed to parse voices.json: " + err.Error())
	}
	voiceToLang = make(map[string]string, 256)
	for lang, voices := range voicesByLang {
		for _, v := range voices {
			voiceToLang[v] = lang
		}
	}
}

// Voices returns the Edge TTS voice catalog as a map from language tag to
// voice IDs. The result is a fresh deep copy on every call; callers may modify
// it freely without affecting the package or other callers.
func Voices() map[string][]string {
	out := make(map[string][]string, len(voicesByLang))
	for lang, ids := range voicesByLang {
		out[lang] = slices.Clone(ids)
	}
	return out
}

// LookupVoiceLang returns the language for a known Edge TTS voice ID.
// Returns ("", false) if the voice is not recognized.
func LookupVoiceLang(voiceID string) (string, bool) {
	lang, ok := voiceToLang[voiceID]
	return lang, ok
}
