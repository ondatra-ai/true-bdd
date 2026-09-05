package steps

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"path/filepath"
	"strconv"

	"github.com/playwright-community/playwright-go"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrGoldenSizeMismatch is returned when a crop and its baseline are different
// sizes, which no per-pixel tolerance can absorb.
var ErrGoldenSizeMismatch = errors.New("the crop and its golden are different sizes")

const (
	// goldensDir is where a crop's committed baseline lives, beside the clause
	// that reads it.
	goldensDir = "goldens"
	// channelSlack is how far one RGBA channel may drift before the pixel counts
	// as different: anti-aliasing moves a channel by a few units, a drift does not.
	channelSlack = 8
	// channelShift converts Go's 16-bit colour channel to the 8 bits the
	// tolerance is stated in.
	channelShift = 8
	// percentScale turns a differing-pixel ratio into the percentage the clause
	// is written in.
	percentScale = 100
)

// registerPixelParitySteps binds the clause comparing a piece of chrome to its
// captured image, and the two vocabularies landing beside it.
func registerPixelParitySteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the "([^"]+)" matches its golden crop within (\d+)% of its `+
		`compared pixels$`, assertGoldenCrop)
	registerBoxCompareSteps(suite)
	registerLayoutShiftSteps(suite)
}

// assertGoldenCrop holds one piece of chrome to the image committed for it. An
// absent baseline is written from this run and the clause passes — which is why
// a crop first minted on a broken build blesses that build.
func assertGoldenCrop(state *State, args []string) error {
	crop := args[0]

	tolerance, err := percentage(state, args[1])
	if err != nil {
		return err
	}

	shot, err := cropScreenshot(state, crop)
	if err != nil {
		return err
	}

	path := goldenPath(state, crop)

	baseline, err := disk.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return mintGolden(state, crop, path, shot)
	}

	if err != nil {
		return state.fail("reading the %q golden %s: %w", crop, path, err)
	}

	return compareCrop(state, crop, tolerance, baseline, shot)
}

// cropSelector maps the name a clause writes onto the testid the UI renders it
// under; a name this table does not know travels as a testid of its own.
func cropSelector(crop string) string {
	switch crop {
	case "breadcrumb":
		return "content-breadcrumb"
	case "chat-toggle":
		return "chat-dock-toggle"
	default:
		return crop
	}
}

// cropScreenshot photographs one named crop, waiting for it exactly as every
// other element clause does.
func cropScreenshot(state *State, crop string) ([]byte, error) {
	sel, locator, err := locateStep(state, cropSelector(crop))
	if err != nil {
		return nil, state.fail("the %q crop: %w", crop, err)
	}

	shot, err := locator.Screenshot(playwright.LocatorScreenshotOptions{
		Animations: playwright.ScreenshotAnimationsDisabled,
	})
	if err != nil {
		return nil, state.fail("photographing the %q crop (%s): %w", crop, sel, err)
	}

	return shot, nil
}

// mintGolden writes the baseline this run captured and says so, so a reader of
// the run's output knows the clause blessed rather than compared.
func mintGolden(state *State, crop, path string, shot []byte) error {
	err := disk.Write(path, shot, disk.Shared)
	if err != nil {
		return state.fail("writing the %q golden %s: %w", crop, path, err)
	}

	state.T.Logf("%s: minted the %q golden at %s from this run — a baseline "+
		"captured on a broken build blesses that build",
		state.Scenario.ID, crop, path)

	return nil
}

// compareCrop grades this run's crop against the committed one, and names both
// sides of whatever it found.
func compareCrop(state *State, crop string, tolerance float64,
	baseline, shot []byte,
) error {
	golden, err := decodePNG(baseline)
	if err != nil {
		return state.fail("decoding the %q golden: %w", crop, err)
	}

	got, err := decodePNG(shot)
	if err != nil {
		return state.fail("decoding the %q crop: %w", crop, err)
	}

	if golden.Bounds().Dx() != got.Bounds().Dx() ||
		golden.Bounds().Dy() != got.Bounds().Dy() {
		return state.fail("%w: the %q crop is %dx%d and its golden is %dx%d",
			ErrGoldenSizeMismatch, crop, got.Bounds().Dx(), got.Bounds().Dy(),
			golden.Bounds().Dx(), golden.Bounds().Dy())
	}

	differing, compared := differingPixels(golden, got)
	if compared == 0 {
		return state.fail("the %q crop is %dx%d, so there is nothing to compare",
			crop, got.Bounds().Dx(), got.Bounds().Dy())
	}

	drift := float64(differing) * percentScale / float64(compared)
	if drift > tolerance {
		return state.fail("the %q crop differs from %s in %d of %d compared "+
			"pixels (%.2f%%), want no more than %.0f%%",
			crop, goldenPath(state, crop), differing, compared, drift, tolerance)
	}

	return nil
}

// differingPixels counts the pixels whose colour moved past the slack, and how
// many were compared — the denominator the clause's percentage is taken over.
func differingPixels(golden, got image.Image) (int, int) {
	bounds := got.Bounds()
	origin := golden.Bounds().Min
	differing, compared := 0, 0

	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for col := bounds.Min.X; col < bounds.Max.X; col++ {
			compared++

			left := golden.At(origin.X+col-bounds.Min.X, origin.Y+row-bounds.Min.Y)
			if pixelDiffers(left, got.At(col, row)) {
				differing++
			}
		}
	}

	return differing, compared
}

// pixelDiffers answers whether two pixels differ on any channel by more than
// the slack.
func pixelDiffers(golden, got color.Color) bool {
	goldenRed, goldenGreen, goldenBlue, goldenAlpha := golden.RGBA()
	gotRed, gotGreen, gotBlue, gotAlpha := got.RGBA()

	return channelDiffers(goldenRed, gotRed) ||
		channelDiffers(goldenGreen, gotGreen) ||
		channelDiffers(goldenBlue, gotBlue) ||
		channelDiffers(goldenAlpha, gotAlpha)
}

// channelDiffers is that comparison on one channel, at the 8 bits the tolerance
// is stated in.
func channelDiffers(golden, got uint32) bool {
	left, right := int(golden>>channelShift), int(got>>channelShift)
	if left > right {
		return left-right > channelSlack
	}

	return right-left > channelSlack
}

// decodePNG reads one captured image.
func decodePNG(data []byte) (image.Image, error) {
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode a PNG: %w", err)
	}

	return decoded, nil
}

// goldenPath is where one crop's baseline is committed.
func goldenPath(state *State, crop string) string {
	return filepath.Join(state.Harness.RepoRoot, "tests", "bdd-web", "steps",
		goldensDir, crop+".png")
}

// percentage parses a clause's percentage, which its own \d+ capture
// guarantees.
func percentage(state *State, text string) (float64, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, state.fail("the step's percentage %q does not parse: %w", text, err)
	}

	return value, nil
}
