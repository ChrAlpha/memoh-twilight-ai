//go:build integration

package images

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/felinics/twilight/internal/testutil"
	"github.com/felinics/twilight/sdk"
)

// Live API tests. Compiled only with -tags=integration; each test skips
// when its credentials are not set. Credentials come from the environment
// or a .env file found by walking up from the package directory.

func TestMain(m *testing.M) {
	testutil.LoadEnv()
	os.Exit(m.Run())
}

func envOrSkip(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("skipping: %s not set", key)
	}
	return v
}

func imageLiveOrSkip(t *testing.T) {
	t.Helper()
	if os.Getenv("DASHSCOPE_IMAGE_LIVE") != "1" {
		t.Skip("skipping: set DASHSCOPE_IMAGE_LIVE=1 to run real image generation tests")
	}
}

func newIntegrationProvider(t *testing.T) *Provider {
	t.Helper()
	imageLiveOrSkip(t)
	apiKey := envOrSkip(t, "DASHSCOPE_API_KEY")
	opts := []Option{WithAPIKey(apiKey)}
	if base := os.Getenv("DASHSCOPE_BASE_URL"); base != "" {
		opts = append(opts, WithBaseURL(base))
	}
	return New(opts...)
}

func integrationEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func TestIntegration_GenerateQwenImage(t *testing.T) {
	provider := newIntegrationProvider(t)
	modelID := integrationEnv("DASHSCOPE_QWEN_IMAGE_MODEL", "qwen-image")

	ctx, cancel := context.WithTimeout(context.Background(), defaultPollTimeout+time.Minute)
	defer cancel()

	result, err := sdk.GenerateImage(ctx,
		sdk.WithImageGenerationModel(provider.GenerationModel(modelID)),
		sdk.WithImagePrompt("A single red cube on a clean white background, no text."),
		sdk.WithImageN(1),
	)
	if err != nil {
		t.Fatalf("GenerateImage(%s): %v", modelID, err)
	}
	assertIntegrationImageURL(t, result)
}

func TestIntegration_GenerateWanImage(t *testing.T) {
	provider := newIntegrationProvider(t)
	modelID := integrationEnv("DASHSCOPE_WAN_IMAGE_MODEL", "wan2.6-t2i")

	ctx, cancel := context.WithTimeout(context.Background(), defaultPollTimeout+time.Minute)
	defer cancel()

	result, err := sdk.GenerateImage(ctx,
		sdk.WithImageGenerationModel(provider.GenerationModel(modelID)),
		sdk.WithImagePrompt("A simple watercolor landscape with one tree beside a lake, no text."),
		sdk.WithImageSize("1024x1024"),
		sdk.WithImageN(1),
	)
	if err != nil {
		t.Fatalf("GenerateImage(%s): %v", modelID, err)
	}
	assertIntegrationImageURL(t, result)
}

func assertIntegrationImageURL(t *testing.T, result *sdk.ImageResult) {
	t.Helper()
	if result == nil {
		t.Fatal("GenerateImage returned nil result")
	}
	if len(result.Data) == 0 {
		t.Fatal("GenerateImage returned no image data")
	}
	if strings.TrimSpace(result.Data[0].URL) == "" && strings.TrimSpace(result.Data[0].B64JSON) == "" {
		t.Fatalf("first image has no URL or b64_json: %+v", result.Data[0])
	}
	t.Logf("image generated: url_set=%t b64_set=%t", strings.TrimSpace(result.Data[0].URL) != "", strings.TrimSpace(result.Data[0].B64JSON) != "")
}
