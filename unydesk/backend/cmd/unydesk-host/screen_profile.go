package main

import "time"

type screenPipelineProfile struct {
	StreamMaxEdge         int
	SignatureColumns      int
	SignatureRows         int
	RefreshUnchangedAfter time.Duration
	FrameInterval         time.Duration
	CaptureRetryDelay     time.Duration
	CaptureFailureBackoff time.Duration
	CaptureErrorCooldown  time.Duration
	KeyframeInterval      time.Duration
	DiffTileSize          int
	PatchAreaThreshold    float64
	Codec                 screenCodecProfile
}

type screenCodecProfile struct {
	RawKeyframeBudgetBytes      int
	RawPatchBudgetBytes         int
	LosslessKeyframeMaxPixels   int
	LosslessKeyframeBudgetBytes int
	LosslessPatchMaxPixels      int
	LosslessPatchBudgetBytes    int
	JPEGQualityBase             int
	JPEGQualitySoft             int
	JPEGQualityHard             int
	JPEGQualityFloor            int
	JPEGSoftBudgetBytes         int
	JPEGHardBudgetBytes         int
	JPEGFloorBudgetBytes        int
	WebPQualityBase             float32
	WebPQualitySoft             float32
	WebPQualityHard             float32
	WebPQualityFloor            float32
	WebPSoftBudgetBytes         int
	WebPHardBudgetBytes         int
	WebPFloorBudgetBytes        int
}

func defaultScreenPipelineProfile() screenPipelineProfile {
	return screenPipelineProfile{
		StreamMaxEdge:         1920,
		SignatureColumns:      32,
		SignatureRows:         18,
		RefreshUnchangedAfter: time.Second,
		FrameInterval:         25 * time.Millisecond,
		CaptureRetryDelay:     35 * time.Millisecond,
		CaptureFailureBackoff: 250 * time.Millisecond,
		CaptureErrorCooldown:  2 * time.Second,
		KeyframeInterval:      4500 * time.Millisecond,
		DiffTileSize:          12,
		PatchAreaThreshold:    0.94,
		Codec: screenCodecProfile{
			RawKeyframeBudgetBytes:      0,
			RawPatchBudgetBytes:         768 * 1024,
			LosslessKeyframeMaxPixels:   0,
			LosslessKeyframeBudgetBytes: 0,
			LosslessPatchMaxPixels:      1400 * 1400,
			LosslessPatchBudgetBytes:    768 * 1024,
			JPEGQualityBase:             84,
			JPEGQualitySoft:             78,
			JPEGQualityHard:             70,
			JPEGQualityFloor:            62,
			JPEGSoftBudgetBytes:         768 * 1024,
			JPEGHardBudgetBytes:         1280 * 1024,
			JPEGFloorBudgetBytes:        2 * 1024 * 1024,
			WebPQualityBase:             82,
			WebPQualitySoft:             74,
			WebPQualityHard:             66,
			WebPQualityFloor:            58,
			WebPSoftBudgetBytes:         768 * 1024,
			WebPHardBudgetBytes:         1280 * 1024,
			WebPFloorBudgetBytes:        2 * 1024 * 1024,
		},
	}
}
