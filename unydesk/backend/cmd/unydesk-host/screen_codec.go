package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/deepteams/webp"
)

type screenImageEncoder struct {
	profile screenCodecProfile
}

func newScreenImageEncoder(profile screenCodecProfile) screenImageEncoder {
	return screenImageEncoder{profile: profile}
}

func (e screenImageEncoder) encode(img image.Image, frameKind byte) ([]byte, string, byte, error) {
	if frameKind == screenWireKindPatch {
		if encoded, ok := encodeRawRGBAWithinBudget(img, e.profile.RawPatchBudgetBytes); ok {
			return encoded, "application/x-unydesk-rgba", screenWireCodecRGBA, nil
		}
		if encoded, contentType, codec, ok, err := e.encodeLosslessWithinBudget(img, e.profile.LosslessPatchMaxPixels, e.profile.LosslessPatchBudgetBytes); err == nil && ok {
			return encoded, contentType, codec, nil
		} else if err != nil {
			return nil, "", 0, err
		}
	} else {
		if encoded, ok := encodeRawRGBAWithinBudget(img, e.profile.RawKeyframeBudgetBytes); ok {
			return encoded, "application/x-unydesk-rgba", screenWireCodecRGBA, nil
		}
		if encoded, contentType, codec, ok, err := e.encodeLosslessWithinBudget(img, e.profile.LosslessKeyframeMaxPixels, e.profile.LosslessKeyframeBudgetBytes); err == nil && ok {
			return encoded, contentType, codec, nil
		} else if err != nil {
			return nil, "", 0, err
		}
	}
	if encoded, err := e.encodeAdaptiveWebP(img); err == nil {
		return encoded, "image/webp", screenWireCodecWebP, nil
	}
	encoded, err := e.encodeAdaptiveJPEG(img)
	if err != nil {
		return nil, "", 0, err
	}
	return encoded, "image/jpeg", screenWireCodecJPEG, nil
}

func encodeRawRGBAWithinBudget(img image.Image, maxBytes int) ([]byte, bool) {
	rgba, ok := img.(*image.RGBA)
	if !ok {
		return nil, false
	}
	bounds := rgba.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, false
	}
	length := width * height * 4
	if length <= 0 || length > maxBytes {
		return nil, false
	}
	if rgba.Stride == width*4 && len(rgba.Pix) >= length {
		return append([]byte(nil), rgba.Pix[:length]...), true
	}

	out := make([]byte, length)
	for row := 0; row < height; row++ {
		srcOffset := row * rgba.Stride
		dstOffset := row * width * 4
		copy(out[dstOffset:dstOffset+width*4], rgba.Pix[srcOffset:srcOffset+width*4])
	}
	return out, true
}

func (e screenImageEncoder) encodeLosslessWithinBudget(img image.Image, maxPixels, maxBytes int) ([]byte, string, byte, bool, error) {
	if encoded, ok, err := encodeLosslessWebPWithinBudget(img, maxPixels, maxBytes); err == nil && ok {
		return encoded, "image/webp", screenWireCodecWebP, true, nil
	} else if err != nil {
		return nil, "", 0, false, err
	}
	if encoded, ok, err := encodePNGWithinBudget(img, maxPixels, maxBytes); err == nil && ok {
		return encoded, "image/png", screenWireCodecPNG, true, nil
	} else if err != nil {
		return nil, "", 0, false, err
	}
	return nil, "", 0, false, nil
}

func encodeLosslessWebPWithinBudget(img image.Image, maxPixels, maxBytes int) ([]byte, bool, error) {
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, false, nil
	}
	if bounds.Dx()*bounds.Dy() > maxPixels {
		return nil, false, nil
	}

	var encoded bytes.Buffer
	options := webp.OptionsForPreset(webp.PresetText, 80)
	options.Lossless = true
	options.Method = 4
	options.Exact = true
	if err := webp.Encode(&encoded, img, options); err != nil {
		return nil, false, err
	}
	if encoded.Len() > maxBytes {
		return nil, false, nil
	}
	return append([]byte(nil), encoded.Bytes()...), true, nil
}

func encodePNGWithinBudget(img image.Image, maxPixels, maxBytes int) ([]byte, bool, error) {
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, false, nil
	}
	if bounds.Dx()*bounds.Dy() > maxPixels {
		return nil, false, nil
	}

	var encoded bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&encoded, img); err != nil {
		return nil, false, err
	}
	if encoded.Len() > maxBytes {
		return nil, false, nil
	}
	return append([]byte(nil), encoded.Bytes()...), true, nil
}

func (e screenImageEncoder) encodeAdaptiveWebP(img image.Image) ([]byte, error) {
	qualities := []float32{e.profile.WebPQualityBase, e.profile.WebPQualitySoft, e.profile.WebPQualityHard, e.profile.WebPQualityFloor}
	budgets := []int{e.profile.WebPSoftBudgetBytes, e.profile.WebPHardBudgetBytes, e.profile.WebPFloorBudgetBytes, 1 << 30}
	var best []byte

	for index, quality := range qualities {
		var encoded bytes.Buffer
		options := webp.OptionsForPreset(webp.PresetText, quality)
		options.Method = 4
		options.Pass = 2
		options.FilterStrength = 0
		options.FilterSharpness = 6
		options.Preprocessing = 0
		options.Exact = true
		if err := webp.Encode(&encoded, img, options); err != nil {
			return nil, err
		}
		best = append(best[:0], encoded.Bytes()...)
		if len(best) <= budgets[index] {
			return append([]byte(nil), best...), nil
		}
	}

	return append([]byte(nil), best...), nil
}

func (e screenImageEncoder) encodeAdaptiveJPEG(img image.Image) ([]byte, error) {
	qualities := []int{e.profile.JPEGQualityBase, e.profile.JPEGQualitySoft, e.profile.JPEGQualityHard, e.profile.JPEGQualityFloor}
	budgets := []int{e.profile.JPEGSoftBudgetBytes, e.profile.JPEGHardBudgetBytes, e.profile.JPEGFloorBudgetBytes, 1 << 30}
	var best []byte

	for index, quality := range qualities {
		var encoded bytes.Buffer
		if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		best = append(best[:0], encoded.Bytes()...)
		if len(best) <= budgets[index] {
			return append([]byte(nil), best...), nil
		}
	}

	return append([]byte(nil), best...), nil
}
