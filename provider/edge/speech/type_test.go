package speech

import (
	"slices"
	"sync"
	"testing"
)

func TestVoices_ReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	first := Voices()
	zh, ok := first["zh-CN"]
	if !ok || !slices.Contains(zh, "zh-CN-XiaoxiaoNeural") {
		t.Fatalf("zh-CN voices missing XiaoxiaoNeural: %v", zh)
	}

	zh[0] = "mutated"
	first["zh-CN"] = nil
	delete(first, "en-US")
	first["xx-XX"] = []string{"xx-XX-FakeNeural"}

	second := Voices()
	if !slices.Contains(second["zh-CN"], "zh-CN-XiaoxiaoNeural") || slices.Contains(second["zh-CN"], "mutated") {
		t.Fatalf("package data changed through returned slice: %v", second["zh-CN"])
	}
	if _, ok := second["en-US"]; !ok {
		t.Fatal("deleting from returned map removed en-US from package data")
	}
	if _, ok := second["xx-XX"]; ok {
		t.Fatal("inserting into returned map leaked into package data")
	}
	if lang, ok := LookupVoiceLang("zh-CN-XiaoxiaoNeural"); !ok || lang != "zh-CN" {
		t.Fatalf("LookupVoiceLang = %q, %v", lang, ok)
	}
}

// TestVoices_ConcurrentReadAndMutate is meaningful under -race: every goroutine
// reads the catalog and mutates its own copy. If Voices returned shared state,
// the race detector (or the runtime's concurrent map write check) would fire.
func TestVoices_ConcurrentReadAndMutate(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				v := Voices()
				for lang, ids := range v {
					if len(ids) > 0 {
						ids[0] = "mutated"
					}
					v[lang] = append(ids, "extra")
				}
				v["goroutine"] = []string{string(rune('a' + i))}
				_, _ = LookupVoiceLang("en-US-EmmaMultilingualNeural")
			}
		}()
	}
	wg.Wait()

	if slices.Contains(Voices()["en-US"], "mutated") {
		t.Fatal("concurrent mutation of returned copies reached package data")
	}
}
